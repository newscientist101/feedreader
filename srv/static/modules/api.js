// Shared fetch wrapper for API calls.

export async function api(method, url, data = null) {
    const options = {
        method,
        headers: {
            'Content-Type': 'application/json',
            'X-Requested-With': 'XMLHttpRequest',
        },
    };
    if (data) {
        options.body = JSON.stringify(data);
    }
    const res = await fetch(url, options);
    if (!res.ok) {
        let message = 'Request failed';
        const text = await res.text();
        try {
            const json = JSON.parse(text);
            message = json.error || message;
        } catch {
            message = text || message;
        }
        throw new Error(message);
    }
    return res.json();
}
