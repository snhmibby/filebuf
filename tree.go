package filebuf

/* A binary tree that holds Data */
type tree struct {
	left, right, parent *tree
	data                data
	size                int64 //left.size + data.size + right.size
}

func newTree(d data) *tree {
	return &tree{data: d, size: d.Size()}
}

//Copy this tree
func (t *tree) Copy() *tree {
	if t == nil {
		return nil
	} else {
		n := *t
		n.data = n.data.Copy()
		n.setLeft(n.left.Copy())
		n.setRight(n.right.Copy())
		return &n
	}
}

/* The set{Left, Right, Parent} functions should be used,
 * because they take into account updating the size field */

func (t *tree) setLeft(l *tree) {
	t.left = l
	if t.left != nil {
		t.left.parent = t
	}
	t.resetSize()
}

func (t *tree) setRight(r *tree) {
	t.right = r
	if t.right != nil {
		t.right.parent = t
	}
	t.resetSize()
}

func (t *tree) setParent(p *tree) {
	t.parent = p
	if t.parent != nil {
		t.parent.resetSize()
	}
}

func (t *tree) resetSize() {
	t.size = treesize(t.left) + t.data.Size() + treesize(t.right)
}

//helper function to query t.size, return 0 on t == nil
func treesize(t *tree) int64 {
	if t != nil {
		return t.size
	}
	return 0
}

func (node *tree) first() *tree {
	n := node
	for n.left != nil {
		n = n.left
	}
	return n
}

func (node *tree) last() *tree {
	n := node
	for n.right != nil {
		n = n.right
	}
	return n
}

func (node *tree) next() *tree {
	n := node
	if n.right != nil {
		n = n.right.first()
	} else {
		for n.parent != nil && n.parent.right == n {
			n = n.parent
		}
		n = n.parent
	}
	return n
}

func (node *tree) prev() *tree {
	n := node
	if n.left != nil {
		n = n.left.last()
	} else {
		for n.parent != nil && n.parent.left == n {
			n = n.parent
		}
		n = n.parent
	}
	return n
}

//get the node that contains the requested offset
func (node *tree) get(offset int64) (*tree, int64) {
	if offset > node.size {
		panic("tree.get; offset > node.size")
	}
	offsetInNode := offset - treesize(node.left)
	nodeSize := node.data.Size()
	switch {
	case offsetInNode < 0:
		return node.left.get(offset)
	case offsetInNode < nodeSize:
		return node, offsetInNode
	default:
		return node.right.get(offsetInNode - nodeSize)
	}
}

type Stats struct {
	size                           int64
	numnodes, filenodes, datanodes int64
	maxdist                        int64   //max distance to root
	avgdist                        float64 //avg distance to root
	maxsz, minsz                   int64   //max/min nodesize
	avgsz                          float64 //average nodesize
}

func updateAvg(avg float64, n_, val_ int64) float64 {
	n := float64(n_)
	val := float64(val_)
	oldsum := avg * n
	return (oldsum + val) / (n + 1)
}

func (t *tree) stats(st *Stats, depth int64) {
	if t != nil {
		t.left.stats(st, depth+1)
		t.right.stats(st, depth+1)
		if t.data.Appendable() {
			st.datanodes++
		} else {
			st.filenodes++
		}
		if depth > st.maxdist {
			st.maxdist = depth
		}
		st.avgdist = updateAvg(st.avgdist, st.numnodes, depth)
		st.avgsz = updateAvg(st.avgsz, st.numnodes, treesize(t))
		tsz := t.data.Size()
		st.size += tsz
		if tsz > st.maxsz {
			st.maxsz = tsz
		}
		if tsz < st.minsz {
			st.minsz = tsz
		}
		st.numnodes++
	}
}

//splay functions from wikipedia
//take care to adjust the size fields

/* Cool ascii art illustration:
 *                        y
 *         x             / \
 *        / \    -->    x   c
 *       a   y         / \
 *          / \       a   b
 *         b   c
 */
func rotateLeft(x *tree) {
	y := x.right
	if y != nil {
		x.setRight(y.left)
		y.setParent(x.parent)
	}
	if x.parent == nil {
	} else if x == x.parent.left {
		x.parent.setLeft(y)
	} else {
		x.parent.setRight(y)
	}
	if y != nil {
		y.setLeft(x)
	}
	x.setParent(y)
}

/* Cool ascii art illustration:
 *                        x
 *         y             / \
 *        / \    <--    y   c
 *       a   x         / \
 *          / \       a   b
 *         b   c
 */
func rotateRight(x *tree) {
	y := x.left
	if y != nil {
		x.setLeft(y.right)
		y.setParent(x.parent)
	}
	if x.parent == nil {
	} else if x == x.parent.right {
		x.parent.setRight(y)
	} else {
		x.parent.setLeft(y)
	}
	if y != nil {
		y.setRight(x)
	}
	x.setParent(y)
}

//see https://en.wikipedia.org/wiki/Splay_tree
func splay(x *tree) *tree {
	for x.parent != nil {
		if x.parent.parent == nil {
			if x == x.parent.left {
				rotateRight(x.parent)
			} else {
				rotateLeft(x.parent)
			}
		} else if x.parent.left == x && x.parent.parent.left == x.parent {
			rotateRight(x.parent.parent)
			rotateRight(x.parent)
		} else if x.parent.right == x && x.parent.parent.right == x.parent {
			rotateLeft(x.parent.parent)
			rotateLeft(x.parent)
		} else if x.parent.left == x && x.parent.parent.right == x.parent {
			rotateRight(x.parent)
			rotateLeft(x.parent)
		} else {
			rotateLeft(x.parent)
			rotateRight(x.parent)
		}
	}
	return x
}
