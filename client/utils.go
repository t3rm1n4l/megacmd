package megaclient

import (
	"github.com/t3rm1n4l/go-mega"
)

// Get all the paths by doing DFS traversal
func getPaths(n *mega.Node, recursive bool) []Path {
	paths := []Path{}
	pathstack := []string{n.GetName()}
	nodestack := []*mega.Node{n}
	consumed := []int{0}

	for len(nodestack) != 0 {
		index := len(nodestack) - 1
		node := nodestack[index]
		next := consumed[index]

		children := []*mega.Node{}
		if recursive {
			children = node.GetChildren()
		}
		switch {
		case next < len(children):
			nodestack = append(nodestack, children[next])
			consumed = append(consumed, 0)
			pathstack = append(pathstack, children[next].GetName())
			consumed[index] = next + 1
		default:
			var p Path
			p.path = make([]string, len(pathstack))
			copy(p.path, pathstack)
			p.t = nodestack[index].GetType()
			p.size = nodestack[index].GetSize()
			p.ts = nodestack[index].GetTimeStamp()
			paths = append(paths, p)

			pathstack = pathstack[:len(pathstack)-1]
			nodestack = nodestack[:index]
			consumed = consumed[:index]

		}
	}

	return paths
}
