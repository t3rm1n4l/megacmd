package mega

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	mrand "math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Default settings
const (
	API_URL              = "https://eu.api.mega.co.nz"
	BASE_DOWNLOAD_URL    = "https://mega.co.nz"
	RETRIES              = 10
	DOWNLOAD_WORKERS     = 3
	MAX_DOWNLOAD_WORKERS = 30
	UPLOAD_WORKERS       = 1
	MAX_UPLOAD_WORKERS   = 30
	TIMEOUT              = time.Second * 10
)

type config struct {
	baseurl    string
	retries    int
	dl_workers int
	ul_workers int
	timeout    time.Duration
}

func newConfig() config {
	return config{
		baseurl:    API_URL,
		retries:    RETRIES,
		dl_workers: DOWNLOAD_WORKERS,
		ul_workers: UPLOAD_WORKERS,
		timeout:    TIMEOUT,
	}
}

// Set mega service base url
func (c *config) SetAPIUrl(u string) {
	if strings.HasSuffix(u, "/") {
		u = strings.TrimRight(u, "/")
	}
	c.baseurl = u
}

// Set number of retries for api calls
func (c *config) SetRetries(r int) {
	c.retries = r
}

// Set concurrent download workers
func (c *config) SetDownloadWorkers(w int) error {
	if w <= MAX_DOWNLOAD_WORKERS {
		c.dl_workers = w
		return nil
	}

	return EWORKER_LIMIT_EXCEEDED
}

// Set connection timeout
func (c *config) SetTimeOut(t time.Duration) {
	c.timeout = t
}

// Set concurrent upload workers
func (c *config) SetUploadWorkers(w int) error {
	if w <= MAX_UPLOAD_WORKERS {
		c.ul_workers = w
		return nil
	}

	return EWORKER_LIMIT_EXCEEDED
}

type Mega struct {
	config
	// Sequence number
	sn int64
	// Server state sn
	ssn string
	// Session ID
	sid []byte
	// Master key
	k []byte
	// User handle
	uh []byte
	// Filesystem object
	FS *MegaFS
	// HTTP Client
	client *http.Client
	// Loggers
	logf   func(format string, v ...interface{})
	debugf func(format string, v ...interface{})
}

// Filesystem node types
const (
	FILE   = 0
	FOLDER = 1
	ROOT   = 2
	INBOX  = 3
	TRASH  = 4
)

// Filesystem node
type Node struct {
	fs       *MegaFS
	name     string
	hash     string
	parent   *Node
	children []*Node
	ntype    int
	size     int64
	ts       time.Time
	meta     NodeMeta
}

func (n *Node) removeChild(c *Node) bool {
	index := -1
	for i, v := range n.children {
		if v.hash == c.hash {
			index = i
			break
		}
	}

	if index >= 0 {
		n.children[index] = n.children[len(n.children)-1]
		n.children = n.children[:len(n.children)-1]
		return true
	}

	return false
}

func (n *Node) addChild(c *Node) {
	if n != nil {
		n.children = append(n.children, c)
	}
}

func (n *Node) getChildren() []*Node {
	return n.children
}

func (n *Node) GetType() int {
	n.fs.mutex.Lock()
	defer n.fs.mutex.Unlock()
	return n.ntype
}

func (n *Node) GetSize() int64 {
	n.fs.mutex.Lock()
	defer n.fs.mutex.Unlock()
	return n.size
}

func (n *Node) GetTimeStamp() time.Time {
	n.fs.mutex.Lock()
	defer n.fs.mutex.Unlock()
	return n.ts
}

func (n *Node) GetName() string {
	n.fs.mutex.Lock()
	defer n.fs.mutex.Unlock()
	return n.name
}

func (n *Node) GetHash() string {
	n.fs.mutex.Lock()
	defer n.fs.mutex.Unlock()
	return n.hash
}

type NodeMeta struct {
	key     []byte
	compkey []byte
	iv      []byte
	mac     []byte
}

// Mega filesystem object
type MegaFS struct {
	root   *Node
	trash  *Node
	inbox  *Node
	sroots []*Node
	lookup map[string]*Node
	skmap  map[string]string
	mutex  sync.Mutex
}

// Get filesystem root node
func (fs *MegaFS) GetRoot() *Node {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()
	return fs.root
}

// Get filesystem trash node
func (fs *MegaFS) GetTrash() *Node {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()
	return fs.trash
}

// Get inbox node
func (fs *MegaFS) GetInbox() *Node {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()
	return fs.inbox
}

// Get a node pointer from its hash
func (fs *MegaFS) HashLookup(h string) *Node {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()

	return fs.hashLookup(h)
}

func (fs *MegaFS) hashLookup(h string) *Node {
	if node, ok := fs.lookup[h]; ok {
		return node
	}

	return nil
}

// Get the list of child nodes for a given node
func (fs *MegaFS) GetChildren(n *Node) ([]*Node, error) {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()

	var empty []*Node

	if n == nil {
		return empty, EARGS
	}

	node := fs.hashLookup(n.hash)
	if node == nil {
		return empty, ENOENT
	}

	return node.getChildren(), nil
}

// Retreive all the nodes in the given node tree path by name
// This method returns array of nodes upto the matched subpath
// (in same order as input names array) even if the target node is not located.
func (fs *MegaFS) PathLookup(root *Node, ns []string) ([]*Node, error) {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()

	if root == nil {
		return nil, EARGS
	}

	var err error
	var found bool = true

	nodepath := []*Node{}

	children := root.children
	for _, name := range ns {
		found = false
		for _, n := range children {
			if n.name == name {
				nodepath = append(nodepath, n)
				children = n.children
				found = true
				break
			}
		}

		if found == false {
			break
		}
	}

	if found == false {
		err = ENOENT
	}

	return nodepath, err
}

// Get top level directory nodes shared by other users
func (fs *MegaFS) GetSharedRoots() []*Node {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()
	return fs.sroots
}

func newMegaFS() *MegaFS {
	fs := &MegaFS{
		lookup: make(map[string]*Node),
		skmap:  make(map[string]string),
	}
	return fs
}

func New() *Mega {
	max := big.NewInt(0x100000000)
	bigx, _ := rand.Int(rand.Reader, max)
	cfg := newConfig()
	mgfs := newMegaFS()
	m := &Mega{
		config: cfg,
		sn:     bigx.Int64(),
		FS:     mgfs,
		client: newHttpClient(cfg.timeout),
	}
	m.SetLogger(log.Printf)
	m.SetDebugger(nil)
	return m
}

// SetClient sets the HTTP client in use
func (m *Mega) SetClient(client *http.Client) *Mega {
	m.client = client
	return m
}

// discardLogf discards the log messages
func discardLogf(format string, v ...interface{}) {
}

// SetLogger sets the logger for important messages.  By default this
// is log.Printf.  Use nil to discard the messages.
func (m *Mega) SetLogger(logf func(format string, v ...interface{})) *Mega {
	if logf == nil {
		logf = discardLogf
	}
	m.logf = logf
	return m
}

// SetDebugger sets the logger for debug messages.  By default these
// messages are not output.
func (m *Mega) SetDebugger(debugf func(format string, v ...interface{})) *Mega {
	if debugf == nil {
		debugf = discardLogf
	}
	m.debugf = debugf
	return m
}

// API request method
func (m *Mega) api_request(r []byte) ([]byte, error) {
	var err error
	var resp *http.Response
	var buf []byte

	defer func() {
		m.sn++
	}()

	url := fmt.Sprintf("%s/cs?id=%d", m.baseurl, m.sn)

	if m.sid != nil {
		url = fmt.Sprintf("%s&sid=%s", url, string(m.sid))
	}

	for i := 0; i < m.retries+1; i++ {
		resp, err = m.client.Post(url, "application/json", bytes.NewBuffer(r))
		if err == nil {
			if resp.StatusCode == 200 {
				goto success
			} else {
				_ = resp.Body.Close()
			}

			err = errors.New("Http Status:" + resp.Status)
		}

		if err != nil {
			continue
		}

	success:
		buf, _ = ioutil.ReadAll(resp.Body)
		err = resp.Body.Close()
		if err != nil {
			return nil, err
		}

		if bytes.HasPrefix(buf, []byte("[")) == false && bytes.HasPrefix(buf, []byte("-")) == false {
			return nil, EBADRESP
		}

		if len(buf) < 6 {
			var emsg [1]ErrorMsg
			err = json.Unmarshal(buf, &emsg)
			if err != nil {
				err = json.Unmarshal(buf, &emsg[0])
			}
			if err != nil {
				return buf, EBADRESP
			}
			err = parseError(emsg[0])
			if err == EAGAIN {
				time.Sleep(time.Millisecond * time.Duration(10))
				continue
			}

			return buf, err
		}

		if err == nil {
			return buf, nil
		}
	}

	return nil, err
}

// Authenticate and start a session
func (m *Mega) Login(email string, passwd string) error {
	var msg [1]LoginMsg
	var res [1]LoginResp
	var err error
	var result []byte

	passkey := password_key(passwd)
	uhandle := stringhash(email, passkey)
	m.uh = make([]byte, len(uhandle))
	copy(m.uh, uhandle)

	msg[0].Cmd = "us"
	msg[0].User = email
	msg[0].Handle = string(uhandle)

	req, _ := json.Marshal(msg)
	result, err = m.api_request(req)

	if err != nil {
		return err
	}

	err = json.Unmarshal(result, &res)
	if err != nil {
		return err
	}

	m.k = base64urldecode([]byte(res[0].Key))
	cipher, err := aes.NewCipher(passkey)
	cipher.Decrypt(m.k, m.k)
	m.sid, err = decryptSessionId([]byte(res[0].Privk), []byte(res[0].Csid), m.k)
	if err != nil {
		return err
	}

	err = m.getFileSystem()

	return err
}

// Get user information
func (m *Mega) GetUser() (UserResp, error) {
	var msg [1]UserMsg
	var res [1]UserResp

	msg[0].Cmd = "ug"

	req, _ := json.Marshal(msg)
	result, err := m.api_request(req)

	if err != nil {
		return res[0], err
	}

	err = json.Unmarshal(result, &res)
	return res[0], err
}

// Add a node into filesystem
func (m *Mega) addFSNode(itm FSNode) (*Node, error) {
	var compkey, key []uint32
	var attr FileAttr
	var node, parent *Node
	var err error

	master_aes, _ := aes.NewCipher(m.k)

	switch {
	case itm.T == FOLDER || itm.T == FILE:
		args := strings.Split(itm.Key, ":")

		switch {
		// File or folder owned by current user
		case args[0] == itm.User:
			buf := base64urldecode([]byte(args[1]))
			err = blockDecrypt(master_aes, buf, buf)
			if err != nil {
				return nil, err
			}
			compkey = bytes_to_a32(buf)
			// Shared folder
		case itm.SUser != "" && itm.SKey != "":
			sk := base64urldecode([]byte(itm.SKey))
			err = blockDecrypt(master_aes, sk, sk)
			if err != nil {
				return nil, err
			}
			sk_aes, _ := aes.NewCipher(sk)

			m.FS.skmap[itm.Hash] = itm.SKey
			buf := base64urldecode([]byte(args[1]))
			err = blockDecrypt(sk_aes, buf, buf)
			if err != nil {
				return nil, err
			}
			compkey = bytes_to_a32(buf)
			// Shared file
		default:
			k := m.FS.skmap[args[0]]
			b := base64urldecode([]byte(k))
			err = blockDecrypt(master_aes, b, b)
			if err != nil {
				return nil, err
			}
			block, _ := aes.NewCipher(b)
			buf := base64urldecode([]byte(args[1]))
			err = blockDecrypt(block, buf, buf)
			if err != nil {
				return nil, err
			}
			compkey = bytes_to_a32(buf)
		}

		switch {
		case itm.T == FILE:
			key = []uint32{compkey[0] ^ compkey[4], compkey[1] ^ compkey[5], compkey[2] ^ compkey[6], compkey[3] ^ compkey[7]}
		default:
			key = compkey
		}

		attr, err = decryptAttr(a32_to_bytes(key), []byte(itm.Attr))
		// FIXME:
		if err != nil {
			attr.Name = "BAD ATTRIBUTE"
		}
	}

	n, ok := m.FS.lookup[itm.Hash]
	switch {
	case ok:
		node = n
	default:
		node = &Node{
			fs:    m.FS,
			ntype: itm.T,
			size:  itm.Sz,
			ts:    time.Unix(itm.Ts, 0),
		}

		m.FS.lookup[itm.Hash] = node
	}

	n, ok = m.FS.lookup[itm.Parent]
	switch {
	case ok:
		parent = n
		parent.removeChild(node)
		parent.addChild(node)
	default:
		parent = nil
		if itm.Parent != "" {
			parent = &Node{
				fs:       m.FS,
				children: []*Node{node},
				ntype:    FOLDER,
			}
			m.FS.lookup[itm.Parent] = parent
		}
	}

	switch {
	case itm.T == FILE:
		var meta NodeMeta
		meta.key = a32_to_bytes(key)
		meta.iv = a32_to_bytes([]uint32{compkey[4], compkey[5], 0, 0})
		meta.mac = a32_to_bytes([]uint32{compkey[6], compkey[7]})
		meta.compkey = a32_to_bytes(compkey)
		node.meta = meta
	case itm.T == FOLDER:
		var meta NodeMeta
		meta.key = a32_to_bytes(key)
		meta.compkey = a32_to_bytes(compkey)
		node.meta = meta
	case itm.T == ROOT:
		attr.Name = "Cloud Drive"
		m.FS.root = node
	case itm.T == INBOX:
		attr.Name = "InBox"
		m.FS.inbox = node
	case itm.T == TRASH:
		attr.Name = "Trash"
		m.FS.trash = node
	}

	// Shared directories
	if itm.SUser != "" && itm.SKey != "" {
		m.FS.sroots = append(m.FS.sroots, node)
	}

	node.name = attr.Name
	node.hash = itm.Hash
	node.parent = parent
	node.ntype = itm.T

	return node, nil
}

// Get all nodes from filesystem
func (m *Mega) getFileSystem() error {
	m.FS.mutex.Lock()
	defer m.FS.mutex.Unlock()

	var msg [1]FilesMsg
	var res [1]FilesResp

	msg[0].Cmd = "f"
	msg[0].C = 1

	req, _ := json.Marshal(msg)
	result, err := m.api_request(req)

	if err != nil {
		return err
	}

	err = json.Unmarshal(result, &res)
	if err != nil {
		return err
	}

	for _, sk := range res[0].Ok {
		m.FS.skmap[sk.Hash] = sk.Key
	}

	for _, itm := range res[0].F {
		_, err = m.addFSNode(itm)
		if err != nil {
			return err
		}
	}

	m.ssn = res[0].Sn

	go m.pollEvents()

	return nil
}

// Download file from filesystem
func (m *Mega) DownloadFile(src *Node, dstpath string, progress *chan int) error {
	m.FS.mutex.Lock()
	defer m.FS.mutex.Unlock()

	defer func() {
		if progress != nil {
			close(*progress)
		}
	}()

	if src == nil {
		return EARGS
	}

	var msg [1]DownloadMsg
	var res [1]DownloadResp
	var outfile *os.File
	var mutex sync.Mutex

	_, err := os.Stat(dstpath)
	if os.IsExist(err) {
		err = os.Remove(dstpath)
		if err != nil {
			return err
		}
	}

	outfile, err = os.OpenFile(dstpath, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return err
	}

	msg[0].Cmd = "g"
	msg[0].G = 1
	msg[0].N = src.hash

	request, _ := json.Marshal(msg)
	result, err := m.api_request(request)
	if err != nil {
		return err
	}

	err = json.Unmarshal(result, &res)
	if err != nil {
		return err
	}
	resourceUrl := res[0].G

	_, err = decryptAttr(src.meta.key, []byte(res[0].Attr))

	aes_block, _ := aes.NewCipher(src.meta.key)

	mac_data := a32_to_bytes([]uint32{0, 0, 0, 0})
	mac_enc := cipher.NewCBCEncrypter(aes_block, mac_data)
	t := bytes_to_a32(src.meta.iv)
	iv := a32_to_bytes([]uint32{t[0], t[1], t[0], t[1]})

	sorted_chunks := []int{}
	chunks := getChunkSizes(int(res[0].Size))
	chunk_macs := make([][]byte, len(chunks))

	for k, _ := range chunks {
		sorted_chunks = append(sorted_chunks, k)
	}
	sort.Ints(sorted_chunks)

	workch := make(chan int)
	errch := make(chan error, m.dl_workers)
	wg := sync.WaitGroup{}

	// Fire chunk download workers
	for w := 0; w < m.dl_workers; w++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			// Wait for work blocked on channel
			for id := range workch {
				var resource *http.Response
				var err error
				mutex.Lock()
				chk_start := sorted_chunks[id]
				chk_size := chunks[chk_start]
				mutex.Unlock()
				chunk_url := fmt.Sprintf("%s/%d-%d", resourceUrl, chk_start, chk_start+chk_size-1)
				for retry := 0; retry < m.retries+1; retry++ {
					resource, err = m.client.Get(chunk_url)
					if err == nil {
						if resource.StatusCode == 200 {
							break
						} else {
							_ = resource.Body.Close()
						}
					}
				}

				var ctr_iv []uint32
				var ctr_aes cipher.Stream
				var chunk []byte

				if err == nil {
					ctr_iv = bytes_to_a32(src.meta.iv)
					ctr_iv[2] = uint32(uint64(chk_start) / 0x1000000000)
					ctr_iv[3] = uint32(chk_start / 0x10)
					ctr_aes = cipher.NewCTR(aes_block, a32_to_bytes(ctr_iv))
					chunk, err = ioutil.ReadAll(resource.Body)
				}

				if err != nil {
					errch <- err
					return
				}
				err = resource.Body.Close()
				if err != nil {
					errch <- err
					return
				}

				ctr_aes.XORKeyStream(chunk, chunk)
				_, err = outfile.WriteAt(chunk, int64(chk_start))
				if err != nil {
					errch <- err
					return
				}

				enc := cipher.NewCBCEncrypter(aes_block, iv)
				i := 0
				block := []byte{}
				chunk = paddnull(chunk, 16)
				for i = 0; i < len(chunk); i += 16 {
					block = chunk[i : i+16]
					enc.CryptBlocks(block, block)
				}

				mutex.Lock()
				if len(chunk_macs) > 0 {
					chunk_macs[id] = make([]byte, 16)
					copy(chunk_macs[id], block)
				}
				mutex.Unlock()

				if progress != nil {
					*progress <- chk_size
				}
			}
		}()
	}

	// Place chunk download jobs to chan
	err = nil
	for id := 0; id < len(chunks) && err == nil; {
		select {
		case workch <- id:
			id++
		case err = <-errch:
		}
	}
	close(workch)

	wg.Wait()

	if err != nil {
		_ = os.Remove(dstpath)
		return err
	}

	for _, v := range chunk_macs {
		mac_enc.CryptBlocks(mac_data, v)
	}

	err = outfile.Close()
	if err != nil {
		return err
	}
	tmac := bytes_to_a32(mac_data)
	if bytes.Equal(a32_to_bytes([]uint32{tmac[0] ^ tmac[1], tmac[2] ^ tmac[3]}), src.meta.mac) == false {
		return EMACMISMATCH
	}

	return nil
}

// Upload a file to the filesystem
func (m *Mega) UploadFile(srcpath string, parent *Node, name string, progress *chan int) (*Node, error) {
	m.FS.mutex.Lock()
	defer m.FS.mutex.Unlock()

	defer func() {
		if progress != nil {
			close(*progress)
		}
	}()

	if parent == nil {
		return nil, EARGS
	}

	var msg [1]UploadMsg
	var res [1]UploadResp
	var cmsg [1]UploadCompleteMsg
	var cres [1]UploadCompleteResp
	var infile *os.File
	var fileSize int64
	var mutex sync.Mutex

	parenthash := parent.hash
	info, err := os.Stat(srcpath)
	if err == nil {
		fileSize = info.Size()
	}

	infile, err = os.OpenFile(srcpath, os.O_RDONLY, 0666)
	if err != nil {
		return nil, err
	}

	msg[0].Cmd = "u"
	msg[0].S = fileSize
	completion_handle := []byte{}

	request, _ := json.Marshal(msg)
	result, err := m.api_request(request)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(result, &res)
	if err != nil {
		return nil, err
	}

	uploadUrl := res[0].P
	ukey := []uint32{0, 0, 0, 0, 0, 0}
	for i, _ := range ukey {
		ukey[i] = uint32(mrand.Int31())

	}

	kbytes := a32_to_bytes(ukey[:4])
	kiv := a32_to_bytes([]uint32{ukey[4], ukey[5], 0, 0})
	aes_block, _ := aes.NewCipher(kbytes)

	mac_data := a32_to_bytes([]uint32{0, 0, 0, 0})
	mac_enc := cipher.NewCBCEncrypter(aes_block, mac_data)
	iv := a32_to_bytes([]uint32{ukey[4], ukey[5], ukey[4], ukey[5]})

	sorted_chunks := []int{}
	chunks := getChunkSizes(int(fileSize))
	chunk_macs := make([][]byte, len(chunks))

	for k, _ := range chunks {
		sorted_chunks = append(sorted_chunks, k)
	}
	sort.Ints(sorted_chunks)

	workch := make(chan int)
	errch := make(chan error, m.ul_workers)
	wg := sync.WaitGroup{}

	// Fire chunk upload workers
	for w := 0; w < m.ul_workers; w++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for id := range workch {
				mutex.Lock()
				chk_start := sorted_chunks[id]
				chk_size := chunks[chk_start]
				mutex.Unlock()
				ctr_iv := bytes_to_a32(kiv)
				ctr_iv[2] = uint32(uint64(chk_start) / 0x1000000000)
				ctr_iv[3] = uint32(chk_start / 0x10)
				ctr_aes := cipher.NewCTR(aes_block, a32_to_bytes(ctr_iv))

				chunk := make([]byte, chk_size)
				n, _ := infile.ReadAt(chunk, int64(chk_start))
				chunk = chunk[:n]

				enc := cipher.NewCBCEncrypter(aes_block, iv)

				i := 0
				block := make([]byte, 16)
				paddedchunk := paddnull(chunk, 16)
				for i = 0; i < len(paddedchunk); i += 16 {
					copy(block[0:16], paddedchunk[i:i+16])
					enc.CryptBlocks(block, block)
				}

				mutex.Lock()
				if len(chunk_macs) > 0 {
					chunk_macs[id] = make([]byte, 16)
					copy(chunk_macs[id], block)
				}
				mutex.Unlock()

				var rsp *http.Response
				var err error
				ctr_aes.XORKeyStream(chunk, chunk)
				chk_url := fmt.Sprintf("%s/%d", uploadUrl, chk_start)
				reader := bytes.NewBuffer(chunk)
				req, _ := http.NewRequest("POST", chk_url, reader)

				chunk_resp := []byte{}
				for retry := 0; retry < m.retries+1; retry++ {
					rsp, err = m.client.Do(req)
					if err == nil {
						if rsp.StatusCode == 200 {
							break
						} else {
							_ = rsp.Body.Close()
						}
					}
				}

				chunk_resp, err = ioutil.ReadAll(rsp.Body)
				if err != nil {
					errch <- err
					return
				}

				err = rsp.Body.Close()
				if err != nil {
					errch <- err
					return
				}

				if bytes.Equal(chunk_resp, nil) == false {
					mutex.Lock()
					completion_handle = chunk_resp
					mutex.Unlock()
				}

				if progress != nil {
					*progress <- chk_size
				}
			}
		}()
	}

	err = nil
	if len(chunks) == 0 {
		// File size is zero
		// Tell single worker to request for completion handle
		sorted_chunks = append(sorted_chunks, 0)
		chunks[0] = 0
		workch <- 0
	} else {
		// Place chunk download jobs to chan
		for id := 0; id < len(chunks) && err == nil; {
			select {
			case workch <- id:
				id++
			case err = <-errch:
			}
		}
	}
	close(workch)

	wg.Wait()

	if err != nil {
		return nil, err
	}

	for _, v := range chunk_macs {
		mac_enc.CryptBlocks(mac_data, v)
	}

	t := bytes_to_a32(mac_data)
	meta_mac := []uint32{t[0] ^ t[1], t[2] ^ t[3]}

	filename := filepath.Base(srcpath)
	if name != "" {
		filename = name
	}
	attr := FileAttr{filename}

	attr_data, _ := encryptAttr(kbytes, attr)

	key := []uint32{ukey[0] ^ ukey[4], ukey[1] ^ ukey[5],
		ukey[2] ^ meta_mac[0], ukey[3] ^ meta_mac[1],
		ukey[4], ukey[5], meta_mac[0], meta_mac[1]}

	buf := a32_to_bytes(key)
	master_aes, _ := aes.NewCipher(m.k)
	iv = a32_to_bytes([]uint32{0, 0, 0, 0})
	enc := cipher.NewCBCEncrypter(master_aes, iv)
	enc.CryptBlocks(buf[:16], buf[:16])
	enc = cipher.NewCBCEncrypter(master_aes, iv)
	enc.CryptBlocks(buf[16:], buf[16:])

	cmsg[0].Cmd = "p"
	cmsg[0].T = parenthash
	cmsg[0].N[0].H = string(completion_handle)
	cmsg[0].N[0].T = FILE
	cmsg[0].N[0].A = string(attr_data)
	cmsg[0].N[0].K = string(base64urlencode(buf))

	request, _ = json.Marshal(cmsg)
	result, err = m.api_request(request)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(result, &cres)
	if err != nil {
		return nil, err
	}
	node, err := m.addFSNode(cres[0].F[0])

	return node, err
}

// Move a file from one location to another
func (m *Mega) Move(src *Node, parent *Node) error {
	m.FS.mutex.Lock()
	defer m.FS.mutex.Unlock()

	if src == nil || parent == nil {
		return EARGS
	}
	var msg [1]MoveFileMsg
	var err error

	msg[0].Cmd = "m"
	msg[0].N = src.hash
	msg[0].T = parent.hash
	msg[0].I, err = randString(10)
	if err != nil {
		return err
	}

	request, _ := json.Marshal(msg)
	_, err = m.api_request(request)

	if err != nil {
		return err
	}

	if src.parent != nil {
		src.parent.removeChild(src)
	}

	parent.addChild(src)
	src.parent = parent

	return nil
}

// Rename a file or folder
func (m *Mega) Rename(src *Node, name string) error {
	m.FS.mutex.Lock()
	defer m.FS.mutex.Unlock()

	if src == nil {
		return EARGS
	}
	var msg [1]FileAttrMsg

	master_aes, _ := aes.NewCipher(m.k)
	attr := FileAttr{name}
	attr_data, _ := encryptAttr(src.meta.key, attr)
	key := make([]byte, len(src.meta.compkey))
	err := blockEncrypt(master_aes, key, src.meta.compkey)
	if err != nil {
		return err
	}

	msg[0].Cmd = "a"
	msg[0].Attr = string(attr_data)
	msg[0].Key = string(base64urlencode(key))
	msg[0].N = src.hash
	msg[0].I, err = randString(10)
	if err != nil {
		return err
	}

	req, _ := json.Marshal(msg)
	_, err = m.api_request(req)

	src.name = name

	return err
}

// Create a directory in the filesystem
func (m *Mega) CreateDir(name string, parent *Node) (*Node, error) {
	m.FS.mutex.Lock()
	defer m.FS.mutex.Unlock()

	if parent == nil {
		return nil, EARGS
	}
	var msg [1]UploadCompleteMsg
	var res [1]UploadCompleteResp

	compkey := []uint32{0, 0, 0, 0, 0, 0}
	for i, _ := range compkey {
		compkey[i] = uint32(mrand.Int31())
	}

	master_aes, _ := aes.NewCipher(m.k)
	attr := FileAttr{name}
	ukey := a32_to_bytes(compkey[:4])
	attr_data, _ := encryptAttr(ukey, attr)
	key := make([]byte, len(ukey))
	err := blockEncrypt(master_aes, key, ukey)
	if err != nil {
		return nil, err
	}

	msg[0].Cmd = "p"
	msg[0].T = parent.hash
	msg[0].N[0].H = "xxxxxxxx"
	msg[0].N[0].T = FOLDER
	msg[0].N[0].A = string(attr_data)
	msg[0].N[0].K = string(base64urlencode(key))
	msg[0].I, err = randString(10)
	if err != nil {
		return nil, err
	}

	req, _ := json.Marshal(msg)
	result, err := m.api_request(req)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(result, &res)
	if err != nil {
		return nil, err
	}
	node, err := m.addFSNode(res[0].F[0])

	return node, err
}

// Delete a file or directory from filesystem
func (m *Mega) Delete(node *Node, destroy bool) error {
	if node == nil {
		return EARGS
	}
	if destroy == false {
		return m.Move(node, m.FS.trash)
	}

	m.FS.mutex.Lock()
	defer m.FS.mutex.Unlock()

	var msg [1]FileDeleteMsg
	var err error
	msg[0].Cmd = "d"
	msg[0].N = node.hash
	msg[0].I, err = randString(10)
	if err != nil {
		return err
	}

	req, _ := json.Marshal(msg)
	_, err = m.api_request(req)

	parent := m.FS.lookup[node.hash]
	parent.removeChild(node)
	delete(m.FS.lookup, node.hash)

	return err
}

// process an add node event
func (m *Mega) processAddNode(evRaw []byte) error {
	m.FS.mutex.Lock()
	defer m.FS.mutex.Unlock()

	var ev FSEvent
	err := json.Unmarshal(evRaw, &ev)
	if err != nil {
		return err
	}

	for _, itm := range ev.T.Files {
		_, err = m.addFSNode(itm)
		if err != nil {
			return err
		}
	}
	return nil
}

// process an update node event
func (m *Mega) processUpdateNode(evRaw []byte) error {
	m.FS.mutex.Lock()
	defer m.FS.mutex.Unlock()

	var ev FSEvent
	err := json.Unmarshal(evRaw, &ev)
	if err != nil {
		return err
	}

	node := m.FS.hashLookup(ev.N)
	attr, err := decryptAttr(node.meta.key, []byte(ev.Attr))
	if err == nil {
		node.name = attr.Name
	} else {
		node.name = "BAD ATTRIBUTE"
	}

	node.ts = time.Unix(ev.Ts, 0)
	return nil
}

// process a delete node event
func (m *Mega) processDeleteNode(evRaw []byte) error {
	m.FS.mutex.Lock()
	defer m.FS.mutex.Unlock()

	var ev FSEvent
	err := json.Unmarshal(evRaw, &ev)
	if err != nil {
		return err
	}

	node := m.FS.hashLookup(ev.N)
	if node != nil && node.parent != nil {
		node.parent.removeChild(node)
		delete(m.FS.lookup, node.hash)
	}
	return nil
}

// Listen for server event notifications and play actions
func (m *Mega) pollEvents() {
	for {
		url := fmt.Sprintf("%s/sc?sn=%s&sid=%s", m.baseurl, m.ssn, string(m.sid))
		resp, err := m.client.Post(url, "application/xml", nil)
		if err != nil {
			time.Sleep(time.Millisecond * 10)
			continue
		}

		if resp.StatusCode != 200 {
			_ = resp.Body.Close()
			continue
		}

		buf, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			time.Sleep(time.Millisecond * 10)
			continue
		}
		err = resp.Body.Close()
		if err != nil {
			m.logf("pollEvents: Error closing body: %v", err)
			continue
		}

		// First attempt to parse an array
		var events Events
		err = json.Unmarshal(buf, &events)
		if err != nil {
			// Try parsing as a lone error message
			var emsg ErrorMsg
			err = json.Unmarshal(buf, &emsg)
			if err != nil {
				m.logf("pollEvents: Bad response received from server: %s", buf)
			} else {
				err = parseError(emsg)
				if err == EAGAIN {
					time.Sleep(time.Millisecond * time.Duration(10))
				} else if err != nil {
					m.logf("pollEvents: Error received from server: %v", err)
				}
			}
			continue
		}

		// if wait URL is set, then fetch it and continue - we
		// don't expect anything else if we have a wait URL.
		if events.W != "" {
			if len(events.E) > 0 {
				m.logf("pollEvents: Unexpected event with w set: %s", buf)
			}
			rsp, err := m.client.Get(events.W)
			if err != nil {
				time.Sleep(time.Millisecond * 10)
			} else {
				_ = rsp.Body.Close()
			}
			continue
		}
		m.ssn = events.Sn

		// For each event in the array, parse it
		for _, evRaw := range events.E {
			// First attempt to unmarshal as an error message
			var emsg ErrorMsg
			err = json.Unmarshal(evRaw, &emsg)
			if err == nil {
				m.logf("pollEvents: Error message received %s", evRaw)
				err = parseError(emsg)
				if err != nil {
					m.logf("pollEvents: Event from server was error: %v", err)
				}
				continue
			}

			// Now unmarshal as a generic event
			var gev GenericEvent
			err = json.Unmarshal(evRaw, &gev)
			if err != nil {
				m.logf("pollEvents: Couldn't parse event from server: %v: %s", err, evRaw)
				continue
			}
			m.debugf("pollEvents: Parsing event %q: %s", gev.Cmd, evRaw)

			// Work out what to do with the event
			var process func([]byte) error
			switch gev.Cmd {
			case "t": // node addition
				process = m.processAddNode
			case "u": // node update
				process = m.processUpdateNode
			case "d": // node deletion
				process = m.processDeleteNode
			case "s", "s2": // share addition/update/revocation
			case "c": // contact addition/update
			case "k": // crypto key request
			case "fa": // file attribute update
			case "ua": // user attribute update
			case "psts": // account updated
			case "ipc": // incoming pending contact request (to us)
			case "opc": // outgoing pending contact request (from us)
			case "upci": // incoming pending contact request update (accept/deny/ignore)
			case "upco": // outgoing pending contact request update (from them, accept/deny/ignore)
			case "ph": // public links handles
			case "se": // set email
			case "mcc": // chat creation / peer's invitation / peer's removal
			case "mcna": // granted / revoked access to a node
			case "uac": // user access control
			default:
				m.debugf("pollEvents: Unknown message %q received: %s", gev.Cmd, evRaw)
			}

			// process the event if we can
			if process != nil {
				err := process(evRaw)
				if err != nil {
					m.logf("pollEvents: Error processing event %q '%s': %v", gev.Cmd, evRaw, err)
				}
			}
		}
	}
}

func (m *Mega) getLink(n *Node) (string, error) {
	m.FS.mutex.Lock()
	defer m.FS.mutex.Unlock()

	var msg [1]GetLinkMsg
	var res [1]string

	msg[0].Cmd = "l"
	msg[0].N = n.hash

	req, _ := json.Marshal(msg)
	result, err := m.api_request(req)

	if err != nil {
		return "", err
	}
	err = json.Unmarshal(result, &res)
	if err != nil {
		return "", err
	}
	return res[0], nil
}

// Exports public link for node, with or without decryption key included
func (m *Mega) Link(n *Node, includeKey bool) (string, error) {
	id, err := m.getLink(n)
	if err != nil {
		return "", err
	}
	if includeKey {
		key := string(base64urlencode(n.meta.compkey))
		return fmt.Sprintf("%v/#!%v!%v", BASE_DOWNLOAD_URL, id, key), nil
	} else {
		return fmt.Sprintf("%v/#!%v", BASE_DOWNLOAD_URL, id), nil
	}
}
