package srv

import (
	"testing"

	"srv.exe.dev/db/dbgen"
)

// ---------------------------------------------------------------------------
// BuildCategoryTree
// ---------------------------------------------------------------------------

func TestBuildCategoryTree_Empty(t *testing.T) {
	roots := BuildCategoryTree(nil)
	if len(roots) != 0 {
		t.Errorf("expected 0 roots, got %d", len(roots))
	}
}

func TestBuildCategoryTree_FlatList(t *testing.T) {
	cats := []dbgen.Category{
		{ID: 1, Name: "Alpha", SortOrder: new(int64(2))},
		{ID: 2, Name: "Beta", SortOrder: new(int64(1))},
		{ID: 3, Name: "Gamma", SortOrder: new(int64(3))},
	}

	roots := BuildCategoryTree(cats)
	if len(roots) != 3 {
		t.Fatalf("expected 3 roots, got %d", len(roots))
	}

	// Should be sorted by sort_order: Beta(1), Alpha(2), Gamma(3)
	expected := []string{"Beta", "Alpha", "Gamma"}
	for i, name := range expected {
		if roots[i].Name != name {
			t.Errorf("roots[%d].Name = %q, want %q", i, roots[i].Name, name)
		}
		if roots[i].Depth != 0 {
			t.Errorf("roots[%d].Depth = %d, want 0", i, roots[i].Depth)
		}
	}
}

func TestBuildCategoryTree_NestedWithDepths(t *testing.T) {
	cats := []dbgen.Category{
		{ID: 1, Name: "Root", SortOrder: new(int64(0))},
		{ID: 2, Name: "Child", ParentID: new(int64(1)), SortOrder: new(int64(0))},
		{ID: 3, Name: "Grandchild", ParentID: new(int64(2)), SortOrder: new(int64(0))},
	}

	roots := BuildCategoryTree(cats)
	if len(roots) != 1 {
		t.Fatalf("expected 1 root, got %d", len(roots))
	}

	root := roots[0]
	if root.Depth != 0 {
		t.Errorf("root depth = %d, want 0", root.Depth)
	}
	if len(root.Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(root.Children))
	}

	child := root.Children[0]
	if child.Depth != 1 {
		t.Errorf("child depth = %d, want 1", child.Depth)
	}
	if child.Name != "Child" {
		t.Errorf("child name = %q, want %q", child.Name, "Child")
	}

	if len(child.Children) != 1 {
		t.Fatalf("expected 1 grandchild, got %d", len(child.Children))
	}
	grandchild := child.Children[0]
	if grandchild.Depth != 2 {
		t.Errorf("grandchild depth = %d, want 2", grandchild.Depth)
	}
}

func TestBuildCategoryTree_OrphanTreatedAsRoot(t *testing.T) {
	// Parent ID 999 doesn't exist — should become a root
	cats := []dbgen.Category{
		{ID: 1, Name: "Normal", SortOrder: new(int64(0))},
		{ID: 2, Name: "Orphan", ParentID: new(int64(999)), SortOrder: new(int64(1))},
	}

	roots := BuildCategoryTree(cats)
	if len(roots) != 2 {
		t.Fatalf("expected 2 roots, got %d", len(roots))
	}
}

func TestBuildCategoryTree_SortByNameWhenSameOrder(t *testing.T) {
	cats := []dbgen.Category{
		{ID: 1, Name: "Zebra", SortOrder: new(int64(0))},
		{ID: 2, Name: "Apple", SortOrder: new(int64(0))},
		{ID: 3, Name: "Mango", SortOrder: new(int64(0))},
	}

	roots := BuildCategoryTree(cats)
	// Same sort_order: should sort by name
	expected := []string{"Apple", "Mango", "Zebra"}
	for i, name := range expected {
		if roots[i].Name != name {
			t.Errorf("roots[%d].Name = %q, want %q", i, roots[i].Name, name)
		}
	}
}

func TestBuildCategoryTree_NilSortOrder(t *testing.T) {
	cats := []dbgen.Category{
		{ID: 1, Name: "B"},
		{ID: 2, Name: "A"},
	}

	roots := BuildCategoryTree(cats)
	// nil sort_order treated as 0; sort by name
	if roots[0].Name != "A" {
		t.Errorf("expected A first, got %q", roots[0].Name)
	}
}

func TestBuildCategoryTree_ChildrenSorted(t *testing.T) {
	cats := []dbgen.Category{
		{ID: 1, Name: "Root", SortOrder: new(int64(0))},
		{ID: 2, Name: "Z-Child", ParentID: new(int64(1)), SortOrder: new(int64(2))},
		{ID: 3, Name: "A-Child", ParentID: new(int64(1)), SortOrder: new(int64(1))},
	}

	roots := BuildCategoryTree(cats)
	children := roots[0].Children
	if len(children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(children))
	}
	if children[0].Name != "A-Child" {
		t.Errorf("children[0] = %q, want A-Child", children[0].Name)
	}
	if children[1].Name != "Z-Child" {
		t.Errorf("children[1] = %q, want Z-Child", children[1].Name)
	}
}

// ---------------------------------------------------------------------------
// FlattenCategoryTree
// ---------------------------------------------------------------------------

func TestFlattenCategoryTree(t *testing.T) {
	cats := []dbgen.Category{
		{ID: 1, Name: "Root", SortOrder: new(int64(0))},
		{ID: 2, Name: "Child", ParentID: new(int64(1)), SortOrder: new(int64(0))},
		{ID: 3, Name: "Grandchild", ParentID: new(int64(2)), SortOrder: new(int64(0))},
		{ID: 4, Name: "Sibling", SortOrder: new(int64(1))},
	}

	roots := BuildCategoryTree(cats)
	flat := FlattenCategoryTree(roots)

	if len(flat) != 4 {
		t.Fatalf("expected 4 nodes, got %d", len(flat))
	}

	// DFS order: Root, Child, Grandchild, Sibling
	expected := []struct {
		name  string
		depth int
	}{
		{"Root", 0},
		{"Child", 1},
		{"Grandchild", 2},
		{"Sibling", 0},
	}
	for i, want := range expected {
		if flat[i].Name != want.name {
			t.Errorf("flat[%d].Name = %q, want %q", i, flat[i].Name, want.name)
		}
		if flat[i].Depth != want.depth {
			t.Errorf("flat[%d].Depth = %d, want %d", i, flat[i].Depth, want.depth)
		}
	}
}

func TestFlattenCategoryTree_Empty(t *testing.T) {
	flat := FlattenCategoryTree(nil)
	if len(flat) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(flat))
	}
}
