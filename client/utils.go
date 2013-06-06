package megaclient

import (
	"github.com/t3rm1n4l/go-mega"
	"strings"
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

func getLookupParams(resource string, fs *mega.MegaFS) (*mega.Node, *[]string, error) {
	resource = strings.TrimSpace(resource)
	args := strings.Split(resource, ":")
	if len(args) != 2 || !strings.HasPrefix(args[1], "/") {
		return nil, nil, EINVALID_PATH
	}

	var root *mega.Node
	var err error

	switch {
	case args[0] == ROOT:
		root = fs.GetRoot()
	case args[0] == TRASH:
		root = fs.GetTrash()
	default:
		return nil, nil, EINVALID_PATH
	}

	pathsplit := strings.Split(args[1], "/")[1:]
	l := len(pathsplit)

	if l > 0 && pathsplit[l-1] == "" {
		pathsplit = pathsplit[:l-1]
		l -= 1
	}

	if l > 0 && pathsplit[l-1] == "" {
		switch {
		case l == 1:
			pathsplit = []string{}
		default:
			pathsplit = pathsplit[:l-2]
		}
	}

	return root, &pathsplit, err
}
