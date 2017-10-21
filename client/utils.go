package megaclient

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/t3rm1n4l/go-humanize"
	"github.com/t3rm1n4l/go-mega"
)

// Get all the paths by doing DFS traversal
func getRemotePaths(fs *mega.MegaFS, n *mega.Node, recursive bool) []Path {
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
			children, _ = fs.GetChildren(node)
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

func getLocalPaths(root string, skiperror bool) ([]Path, error) {
	var paths []Path

	walker := func(p string, info os.FileInfo, err error) error {
		var x Path
		p, _ = filepath.Rel(root, p)

		if err != nil {
			if skiperror {
				return nil
			} else {
				return err
			}
		}

		if p == "." {
			return nil
		}

		x.path = strings.Split(p, "/")
		switch {
		case info.IsDir():
			x.t = mega.FOLDER
			// Go 1.0 compatibility
		case info.Mode()&os.ModeType == 0:
			x.t = mega.FILE
		default:
			return nil
		}

		x.size = info.Size()
		paths = append(paths, x)

		return nil
	}

	err := filepath.Walk(root, walker)

	return paths, err
}

func getLookupParams(resource string, fs *mega.MegaFS) (*mega.Node, *[]string, error) {
	resource = strings.TrimSpace(resource)
	args := strings.SplitN(resource, ":", 2)
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

func RoundDuration(d time.Duration) time.Duration {
	return time.Second * time.Duration(int(d.Seconds()))
}

func progressBar(ch chan int, wg *sync.WaitGroup, size int64, src, dst string) {
	defer func() {
		fmt.Println()
		wg.Done()
	}()
	bytesread := 0
	bps := uint64(0)
	percent := float32(0)
	elapsed := time.Duration(0)
	dur := time.Duration(0)
	var lastLineNum int
	var fmtStr string
	var isWin = runtime.GOOS == "windows"
	if isWin {
		// windows not support ascii escape code
		fmtStr = "\rCopying %s -> %s # %.2f %% of %s at %.4s/s %v "
	} else {
		fmtStr = "\r\033[2KCopying %s -> %s # %.2f %% of %s at %.4s/s %v "
	}

	showProgress := func() {
		if isWin {
			// so just print space to clear last line.
			fmt.Fprintf(os.Stdout, "\r%s", bytes.Repeat([]byte{0x20}, lastLineNum))
		}
		lastLineNum, _ = fmt.Fprintf(os.Stdout, fmtStr, src, dst, percent, humanize.Bytes(uint64(size)), humanize.Bytes(bps), dur)
	}

	showProgress()
	start := time.Now()
	for {
		b := 0
		ok := false

		select {
		case b, ok = <-ch:
			if ok == false {
				return
			}

		case <-time.After(time.Second):
			elapsed = time.Now().Sub(start)
			dur = RoundDuration(elapsed)
			showProgress()
			continue

		}
		bytesread += b
		elapsed = time.Now().Sub(start)
		bps = uint64(float64(bytesread) / elapsed.Seconds())
		percent = 100 * float32(bytesread) / float32(size)
		dur = RoundDuration(elapsed)
		showProgress()
	}
}
