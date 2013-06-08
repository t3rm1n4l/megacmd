package megaclient

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/t3rm1n4l/go-mega"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

const (
	PATH_WIDTH = 50
	SIZE_WIDTH = 10
)

type MegaClient struct {
	cfg  *Config
	mega *mega.Mega
}

type Config struct {
	BaseUrl         string
	Retries         int
	DownloadWorkers int
	UploadWorkers   int
	TimeOut         int
	User            string
	Password        string
	Recursive       bool
	Force           bool
	Verbose         int
}

type Path struct {
	prefix string
	path   []string
	size   int64
	t      int
	ts     time.Time
}

func (p *Path) SetPrefix(s string) {
	p.prefix = s
}

func (p Path) GetPath() string {
	x := path.Join(p.path...)
	x = path.Join(p.prefix, x)
	if p.t == mega.FOLDER {
		x = x + "/"
	}

	return x
}

func (p Path) String() string {
	return fmt.Sprintf("%-*s %-*d %s", PATH_WIDTH, p.GetPath(), SIZE_WIDTH, p.size, p.ts.Format(time.RFC3339))
}

const (
	ROOT  = "mega"
	TRASH = "trash"
)

var (
	EINVALID_CONFIG = errors.New("Invalid json config")
	EINVALID_PATH   = errors.New("Invalid mega path")
	ENOT_FILE       = errors.New("Requested object is not a file")
	EINVALID_DEST   = errors.New("Invalid destination path")
	EINVALID_SRC    = errors.New("Invalid source path")
	EINVALID_SYNC   = errors.New("Invalid sync command parameters")
	ENOT_DIRECTORY  = errors.New("A non-directory exists at this path")
	EFILE_EXISTS    = errors.New("File with same name already exists")
)

func (cfg *Config) Parse(path string) error {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, cfg)

	if err != nil {
		return EINVALID_CONFIG
	}

	return nil
}

func NewMegaClient(conf *Config) *MegaClient {
	log.SetFlags(0)
	c := &MegaClient{
		cfg:  conf,
		mega: mega.New(),
	}

	if conf.BaseUrl != "" {
		c.mega.SetAPIUrl(conf.BaseUrl)
	}

	if conf.Retries != 0 {
		c.mega.SetRetries(conf.Retries)
	}

	if conf.DownloadWorkers != 0 {
		c.mega.SetDownloadWorkers(conf.DownloadWorkers)
	}

	if conf.UploadWorkers != 0 {
		c.mega.SetUploadWorkers(conf.UploadWorkers)
	}

	if conf.TimeOut != 0 {
		c.mega.SetTimeOut(time.Duration(conf.TimeOut) * time.Second)
	}

	return c
}

func (mc *MegaClient) Login() error {
	err := mc.mega.Login(mc.cfg.User, mc.cfg.Password)

	if err != nil {
		return err
	}

	return mc.mega.GetFileSystem()

}

func (mc *MegaClient) List(resource string) (*[]Path, error) {
	var root *mega.Node
	var paths []Path
	var err error

	root, pathsplit, err := getLookupParams(resource, mc.mega.FS)
	if err != nil {
		return nil, err
	}

	var nodes []*mega.Node
	if len(*pathsplit) > 0 {
		nodes, err = mc.mega.FS.PathLookup(root, *pathsplit)
	}

	if err == nil {
		l := len(nodes)

		switch {
		case len(*pathsplit) == 0:
			nodes = root.GetChildren()
		case l > 0:
			nodes = nodes[l-1:]
			if len(nodes) == 1 {
				nodes = nodes[0].GetChildren()
			}
		}

		for _, n := range nodes {
			for _, p := range getRemotePaths(n, mc.cfg.Recursive) {
				p.SetPrefix(resource)
				paths = append(paths, p)
			}
		}
		return &paths, nil
	}

	return nil, err
}

func (mc *MegaClient) Delete(resource string) error {
	root, pathsplit, err := getLookupParams(resource, mc.mega.FS)
	if err != nil {
		return err
	}

	var nodes []*mega.Node
	if len(*pathsplit) > 0 {
		nodes, err = mc.mega.FS.PathLookup(root, *pathsplit)
	} else {
		err = EINVALID_PATH
	}

	if err != nil {
		return err
	}

	l := len(nodes)
	node := nodes[l-1]

	return mc.mega.Delete(node, mc.cfg.Force)
}

func (mc *MegaClient) Move(srcres, dstres string) error {
	root, pathsplit, err := getLookupParams(srcres, mc.mega.FS)
	if err != nil {
		return err
	}

	var nodes []*mega.Node
	var srcnode, dstnode *mega.Node
	var name string
	if len(*pathsplit) > 0 {
		nodes, err = mc.mega.FS.PathLookup(root, *pathsplit)
	} else {
		err = EINVALID_PATH
	}

	if err != nil {
		return err
	}

	srcnode = nodes[len(nodes)-1]

	root, pathsplit, err = getLookupParams(dstres, mc.mega.FS)
	if err != nil {
		return err
	}

	if len(*pathsplit) > 0 {
		nodes, err = mc.mega.FS.PathLookup(root, *pathsplit)
	} else {
		err = EINVALID_PATH
	}

	if err != nil && err != mega.ENOENT {
		return err
	}

	lp := len(*pathsplit)
	ln := len(nodes)

	var rename bool
	switch {
	case lp == ln:
		dstnode = nodes[ln-1]
		rename = false
	case lp == ln+1:
		if ln == 0 {
			dstnode = root
		} else {
			dstnode = nodes[ln-1]
		}
		name = (*pathsplit)[lp-1]
		rename = true
	default:
		return err
	}

	// FIXME: auto fs update
	mc.mega.GetFileSystem()

	err = mc.mega.Move(srcnode, dstnode)

	if err != nil {
		return err
	}

	if rename {
		err = mc.mega.Rename(srcnode, name)
	}

	return err
}

func (mc *MegaClient) Get(srcres, dstpath string) error {
	root, pathsplit, err := getLookupParams(srcres, mc.mega.FS)
	if err != nil {
		return err
	}

	var nodes []*mega.Node
	var node *mega.Node

	fi, err := os.Stat(dstpath)
	if os.IsNotExist(err) {
		d := path.Dir(dstpath)
		fi, err := os.Stat(d)
		if os.IsNotExist(err) {
			return EINVALID_DEST
		} else {
			if !fi.Mode().IsDir() {
				return EINVALID_DEST
			}
		}
	} else {
		if fi.Mode().IsDir() {
			dstpath = path.Join(dstpath, (*pathsplit)[len(*pathsplit)-1])
		} else {
			if mc.cfg.Force {
				err = os.Remove(dstpath)
				if err != nil {
					return err
				}
			} else {
				return EFILE_EXISTS
			}
		}
		err = nil
	}

	if len(*pathsplit) > 0 {
		nodes, err = mc.mega.FS.PathLookup(root, *pathsplit)
	} else {
		err = EINVALID_PATH
	}

	if err != nil {
		return err
	} else {
		node = nodes[len(nodes)-1]
		if node.GetType() != mega.FILE {
			return ENOT_FILE
		}
	}

	var ch *chan int
	var wg sync.WaitGroup
	if mc.cfg.Verbose > 0 {
		ch = new(chan int)
		*ch = make(chan int)

		wg.Add(1)
		go progressBar(*ch, &wg, node.GetSize(), srcres, dstpath)
	}

	err = mc.mega.DownloadFile(node, dstpath, ch)
	wg.Wait()
	return err
}

func (mc *MegaClient) Put(srcpath, dstres string) error {
	var nodes []*mega.Node
	var node *mega.Node
	_, err := os.Stat(srcpath)

	if err != nil {
		return EINVALID_SRC
	}

	root, pathsplit, err := getLookupParams(dstres, mc.mega.FS)
	if err != nil {
		return err
	}
	if len(*pathsplit) > 0 {
		nodes, err = mc.mega.FS.PathLookup(root, *pathsplit)
	}

	if err != nil && err != mega.ENOENT {
		return err
	}

	lp := len(*pathsplit)
	ln := len(nodes)

	var name string
	switch {
	case lp == ln+1 || ln == 0:
		if ln == 0 {
			node = root
			x := strings.Split(dstres, "/")
			if len(x) > 0 {
				name = x[len(x)-1]
			}
		} else {
			node = nodes[ln-1]
			name = (*pathsplit)[lp-1]
		}

	case lp == ln:
		name = path.Base(srcpath)
		node = nodes[ln-1]
	default:
		return err
	}

	if node.GetType() == mega.FILE {
		if len(nodes) > 1 {
			node = nodes[ln-2]
		} else {
			node = root
		}
	}

	for _, c := range node.GetChildren() {
		if c.GetName() == name {
			if mc.cfg.Force {
				err = mc.mega.Delete(c, false)
				if err != nil {
					return err
				}
				if err != nil {
					return err
				}
			} else {
				return EFILE_EXISTS
			}
		}
	}

	var ch *chan int
	var wg sync.WaitGroup
	if mc.cfg.Verbose > 0 {
		ch = new(chan int)
		*ch = make(chan int)
		fi, err := os.Stat(srcpath)
		if err != nil {
			return err
		}

		wg.Add(1)
		go progressBar(*ch, &wg, fi.Size(), srcpath, dstres)
	}

	_, err = mc.mega.UploadFile(srcpath, node, name, ch)
	wg.Wait()
	return err
}

func (mc *MegaClient) Mkdir(dstres string) error {
	var nodes []*mega.Node
	var node *mega.Node

	root, pathsplit, err := getLookupParams(dstres, mc.mega.FS)
	if err != nil {
		return err
	}
	if len(*pathsplit) > 0 {
		nodes, err = mc.mega.FS.PathLookup(root, *pathsplit)
	} else {
		return nil
	}

	lp := len(*pathsplit)
	ln := len(nodes)

	if len(nodes) > 0 {
		node = nodes[ln-1]
	} else {
		node = root
	}

	switch {
	case err == nil:
		if node.GetType() != mega.FOLDER {
			return ENOT_DIRECTORY
		}
		return nil
	case err == mega.ENOENT:
		remaining := lp - ln
		for i := 0; i < remaining; i++ {
			name := (*pathsplit)[ln]
			node, err = mc.mega.CreateDir(name, node)
			if err != nil {
				return err
			}
			ln += 1
		}
		err = nil

	default:
		return err
	}

	return nil
}

func (mc *MegaClient) Sync(src, dst string) error {
	var srcremote bool
	var paths []Path
	root, pathsplits, err := getLookupParams(src, mc.mega.FS)
	switch {
	case err == EINVALID_PATH:
		r, ps, e := getLookupParams(dst, mc.mega.FS)
		if e != nil {
			return EINVALID_SYNC
		}
		root = r
		pathsplits = ps
		err = e
		srcremote = false
	case err == nil:
		_, _, e := getLookupParams(dst, mc.mega.FS)
		if e != EINVALID_PATH {
			return EINVALID_SYNC
		}
		err = e
		srcremote = true
	default:
		return err
	}

	if srcremote {
		var node *mega.Node
		nodes, err := mc.mega.FS.PathLookup(root, *pathsplits)
		if err != nil {
			return err
		}
		if len(nodes) > 0 {
			node = nodes[len(nodes)-1]
		} else {
			node = root
		}
		for _, n := range node.GetChildren() {
			paths = append(paths, getRemotePaths(n, true)...)
		}
	} else {
		paths, err = getLocalPaths(src)
		if err != nil {
			return err
		}
	}

	if mc.cfg.Verbose > 0 {
		log.Printf("Found %d file(s) to be copied", len(paths))
	}

	for _, spath := range paths {
		suffix := spath.GetPath()
		x := path.Join(src, suffix)
		y := path.Join(dst, suffix)

		dir := y
		if spath.t == mega.FILE {
			dir = path.Dir(y)
		}

		if srcremote {
			err = os.MkdirAll(dir, os.ModePerm)
			if err != nil {
				return err
			}
			if spath.t == mega.FILE {
				err = mc.Get(x, y)
			}
		} else {
			err = mc.Mkdir(dir)
			if err != nil {
				return err
			}

			// FIXME: remove on imp of autosync
			err = mc.mega.GetFileSystem()
			if err != nil {
				return err
			}

			if spath.t == mega.FILE {
				err = mc.Put(x, y)
			}
		}
		if err != nil {
			return err
		}
	}

	return nil
}
