package srv

import (
	"srv.exe.dev/db/dbgen"
)

// CategoryNode represents a category with its children
type CategoryNode struct {
	dbgen.Category
	Children []*CategoryNode `json:"children"`
	Depth    int             `json:"depth"`
}

// BuildCategoryTree organizes categories into a tree structure
func BuildCategoryTree(categories []dbgen.Category) []*CategoryNode {
	// Create a map for quick lookup
	nodeMap := make(map[int64]*CategoryNode)
	for i := range categories {
		nodeMap[categories[i].ID] = &CategoryNode{
			Category: categories[i],
			Children: []*CategoryNode{},
			Depth:    0,
		}
	}

	// Build tree by connecting parents and children
	var roots []*CategoryNode
	for _, node := range nodeMap {
		if node.ParentID == nil {
			roots = append(roots, node)
		} else {
			parent, exists := nodeMap[*node.ParentID]
			if exists {
				parent.Children = append(parent.Children, node)
			} else {
				// Parent doesn't exist, treat as root
				roots = append(roots, node)
			}
		}
	}

	// Sort roots and children, then calculate depths
	sortCategoryNodes(roots)
	for _, root := range roots {
		calculateDepths(root, 0)
	}

	return roots
}

func sortCategoryNodes(nodes []*CategoryNode) {
	// Sort by sort_order, then by name
	for i := 0; i < len(nodes)-1; i++ {
		for j := i + 1; j < len(nodes); j++ {
			iOrder := int64(0)
			jOrder := int64(0)
			if nodes[i].SortOrder != nil {
				iOrder = *nodes[i].SortOrder
			}
			if nodes[j].SortOrder != nil {
				jOrder = *nodes[j].SortOrder
			}
			if iOrder > jOrder || (iOrder == jOrder && nodes[i].Name > nodes[j].Name) {
				nodes[i], nodes[j] = nodes[j], nodes[i]
			}
		}
	}
	// Recursively sort children
	for _, node := range nodes {
		sortCategoryNodes(node.Children)
	}
}

func calculateDepths(node *CategoryNode, depth int) {
	node.Depth = depth
	for _, child := range node.Children {
		calculateDepths(child, depth+1)
	}
}

// FlattenCategoryTree returns categories in display order with depth info
func FlattenCategoryTree(roots []*CategoryNode) []*CategoryNode {
	var result []*CategoryNode
	for _, root := range roots {
		flattenNode(root, &result)
	}
	return result
}

func flattenNode(node *CategoryNode, result *[]*CategoryNode) {
	*result = append(*result, node)
	for _, child := range node.Children {
		flattenNode(child, result)
	}
}
