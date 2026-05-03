package usenet_test

import (
	"errors"
	"testing"

	"github.com/newscientist101/feedreader/srv/usenet"
)

func TestCheckArticleBinary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		headers map[string]string
		subject string
		body    string
		wantErr bool // true = ErrBinaryPost expected
	}{
		// --- Accepted plain-text articles ---
		{
			name:    "plain text no content-type",
			headers: map[string]string{},
			subject: "Re: Has anyone read The Dispossessed?",
			body:    "Yes, it's a classic Ursula Le Guin novel.",
			wantErr: false,
		},
		{
			name:    "explicit text/plain",
			headers: map[string]string{"Content-Type": "text/plain; charset=utf-8"},
			subject: "Go 1.22 is out",
			body:    "Just upgraded today.",
			wantErr: false,
		},
		{
			name: "quoted-printable text/plain accepted",
			headers: map[string]string{
				"Content-Type":              "text/plain",
				"Content-Transfer-Encoding": "quoted-printable",
			},
			subject: "Test",
			body:    "Hello world.",
			wantErr: false,
		},

		// --- Rejected by Content-Type ---
		{
			name:    "text/html rejected",
			headers: map[string]string{"Content-Type": "text/html"},
			subject: "Some HTML post",
			body:    "<html><body>hi</body></html>",
			wantErr: true,
		},
		{
			name:    "multipart rejected",
			headers: map[string]string{"Content-Type": "multipart/mixed; boundary=abc"},
			subject: "Attachment post",
			body:    "--abc\r\n...",
			wantErr: true,
		},
		{
			name:    "application/octet-stream rejected",
			headers: map[string]string{"Content-Type": "application/octet-stream"},
			subject: "Binary file",
			body:    "\x00\x01\x02",
			wantErr: true,
		},
		{
			name:    "image/jpeg rejected",
			headers: map[string]string{"Content-Type": "image/jpeg"},
			subject: "Cool photo",
			body:    "...",
			wantErr: true,
		},
		{
			name:    "audio/mpeg rejected",
			headers: map[string]string{"Content-Type": "audio/mpeg"},
			subject: "Song",
			body:    "...",
			wantErr: true,
		},
		{
			name:    "video/mp4 rejected",
			headers: map[string]string{"Content-Type": "video/mp4"},
			subject: "Video clip",
			body:    "...",
			wantErr: true,
		},

		// --- Rejected by Content-Disposition ---
		{
			name: "attachment disposition rejected",
			headers: map[string]string{
				"Content-Type":        "text/plain",
				"Content-Disposition": "attachment; filename=\"file.txt\"",
			},
			subject: "Some attachment",
			body:    "data",
			wantErr: true,
		},

		// --- Rejected by Content-Transfer-Encoding: base64 ---
		{
			name: "base64 transfer encoding rejected",
			headers: map[string]string{
				"Content-Type":              "text/plain",
				"Content-Transfer-Encoding": "base64",
			},
			subject: "Encoded post",
			body:    "aGVsbG8gd29ybGQ=",
			wantErr: true,
		},

		// --- Rejected by yEnc body marker ---
		{
			name:    "yenc body rejected",
			headers: map[string]string{},
			subject: "Some file",
			body:    "=ybegin part=1 line=128 size=12345 name=file.rar\n...",
			wantErr: true,
		},

		// --- Rejected by binary subject patterns ---
		{
			name:    "subject with yEnc keyword",
			headers: map[string]string{},
			subject: "Great.Video.S01E01.yEnc [1/23]",
			body:    "regular text body",
			wantErr: true,
		},
		{
			name:    "subject with [N/M] part indicator",
			headers: map[string]string{},
			subject: "GreatFile.rar [03/23]",
			body:    "some body",
			wantErr: true,
		},
		{
			name:    "subject with (N of M) part indicator",
			headers: map[string]string{},
			subject: "Archive.zip (1 of 5)",
			body:    "some body",
			wantErr: true,
		},
		{
			name:    "subject with (N/M) part indicator in parens",
			headers: map[string]string{},
			subject: "File.nzb (01/23)",
			body:    "some body",
			wantErr: true,
		},
		{
			name:    "innocent subject not rejected",
			headers: map[string]string{},
			subject: "Re: Part 1 of my series [interesting]",
			body:    "Just a discussion.",
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := usenet.CheckArticleBinary(tc.headers, tc.subject, tc.body)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected ErrBinaryPost, got nil")
				}
				if !errors.Is(err, usenet.ErrBinaryPost) {
					t.Fatalf("expected ErrBinaryPost, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
