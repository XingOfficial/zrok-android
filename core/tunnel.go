package zrokcore
import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
	"github.com/openziti/sdk-golang/ziti"
	"github.com/openziti/zrok/v2/environment"
	"github.com/openziti/zrok/v2/sdk/golang/sdk"
	"golang.org/x/sys/unix"
)
type tunnelInfo struct {
	Id         string `json:"id"`
	Mode       string `json:"mode"`
	ShareToken string `json:"shareToken"`
	Target     string `json:"target"`
	State      string `json:"state"`
	Rx         uint64 `json:"rx"`
	Tx         uint64 `json:"tx"`
	Conns      int32  `json:"conns"`
	StartedAt  int64  `json:"startedAt"`
	LastError  string `json:"lastError"`
}
type tunnelCtl struct {
	id, mode, shareToken, target string
	startedAt                    int64
	listener                     io.Closer
	run                          atomic.Bool
	conns                        atomic.Int32
	rx, tx                       atomic.Uint64
	lastErr                      atomic.Value
}
func (t *tunnelCtl) snapshot() tunnelInfo {
	state := "running"
	if !t.run.Load() {
		state = "stopped"
	}
	le, _ := t.lastErr.Load().(string)
	return tunnelInfo{t.id, t.mode, t.shareToken, t.target, state,
		t.rx.Load(), t.tx.Load(), t.conns.Load(), t.startedAt, le}
}
var tunnels sync.Map
var seq atomic.Int64
var zctxs sync.Map
func nextId() string { return strconv.FormatInt(seq.Add(1), 10) }
func zctxFor(rootDir string) (*ziti.Context, error) {
	if v, ok := zctxs.Load(rootDir); ok {
		return v.(*ziti.Context), nil
	}
	environment.SetRootDirName(rootDir)
	root, err := environment.LoadRoot()
	if err != nil {
		return nil, err
	}
	zif, err := root.ZitiIdentityNamed(root.EnvironmentIdentityName())
	if err != nil {
		return nil, err
	}
	zcfg, err := ziti.NewConfigFromFile(zif)
	if err != nil {
		return nil, err
	}
	ctx, err := ziti.NewContext(zcfg)
	if err != nil {
		return nil, err
	}
	zctxs.Store(rootDir, &ctx)
	return &ctx, nil
}
type countedZiti struct {
	net.Conn
	ctl *tunnelCtl
}
func (c *countedZiti) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	c.ctl.rx.Add(uint64(n))
	return n, err
}
func (c *countedZiti) Write(p []byte) (int, error) {
	n, err := c.Conn.Write(p)
	c.ctl.tx.Add(uint64(n))
	return n, err
}
func pumpPair(remote *countedZiti, local net.Conn) {
	go func() {
		io.Copy(remote, local)
		remote.Close()
		local.Close()
	}()
	io.Copy(local, remote)
	local.Close()
	remote.Close()
}
func StartHost(rootDir, shrToken, target string) string {
	id := nextId()
	ctl := &tunnelCtl{id: id, mode: "host", shareToken: shrToken,
		target: target, startedAt: time.Now().Unix()}
	tunnels.Store(id, ctl)
	ctl.run.Store(true)
	go func() {
		environment.SetRootDirName(rootDir)
		root, err := environment.LoadRoot()
		if err != nil {
			ctl.lastErr.Store(err.Error())
			ctl.run.Store(false)
			return
		}
		l, err := sdk.NewListener(shrToken, root)
		if err != nil {
			ctl.lastErr.Store(err.Error())
			ctl.run.Store(false)
			return
		}
		ctl.listener = l
		for ctl.run.Load() {
			c, err := l.Accept()
			if err != nil {
				if ctl.run.Load() {
					ctl.lastErr.Store(err.Error())
				}
				return
			}
			ctl.conns.Add(1)
			go func(zc net.Conn) {
				defer ctl.conns.Add(-1)
				up, err := net.DialTimeout("tcp", target, 5*time.Second)
				if err != nil {
					ctl.lastErr.Store(err.Error())
					zc.Close()
					return
				}
				pumpPair(&countedZiti{Conn: zc, ctl: ctl}, up)
			}(c)
		}
	}()
	return jok(id)
}
func recvFD(c *net.UnixConn) (int, error) {
	buf := make([]byte, 32)
	oob := make([]byte, 128)
	n, _, _, _, err := c.ReadMsgUnix(buf, oob)
	if err != nil {
		return -1, err
	}
	scms, err := unix.ParseSocketControlMessage(oob[:n])
	if err != nil {
		return -1, err
	}
	for _, scm := range scms {
		fds, err := unix.ParseRights(scm)
		if err != nil {
			continue
		}
		if len(fds) > 0 {
			return fds[0], nil
		}
	}
	return -1, errors.New("no fd received")
}
func StartAccess(rootDir, shrToken, sockName string, port int) string {
	id := nextId()
	ctl := &tunnelCtl{id: id, mode: "access", shareToken: shrToken,
		target: "127.0.0.1:" + strconv.Itoa(port), startedAt: time.Now().Unix()}
	tunnels.Store(id, ctl)
	ctl.run.Store(true)
	go func() {
		zctx, err := zctxFor(rootDir)
		if err != nil {
			ctl.lastErr.Store(err.Error())
			ctl.run.Store(false)
			return
		}
		ul, err := net.Listen("unix", "\000"+sockName)
		if err != nil {
			ctl.lastErr.Store(err.Error())
			ctl.run.Store(false)
			return
		}
		ctl.listener = ul
		for ctl.run.Load() {
			c, err := ul.Accept()
			if err != nil {
				if ctl.run.Load() {
					ctl.lastErr.Store(err.Error())
				}
				return
			}
			uc, ok := c.(*net.UnixConn)
			if !ok {
				c.Close()
				continue
			}
			fd, err := recvFD(uc)
			uc.Close()
			if err != nil {
				ctl.lastErr.Store(err.Error())
				continue
			}
			ctl.conns.Add(1)
			go func(fdesc int) {
				defer ctl.conns.Add(-1)
				lc, err := net.FileConn(os.NewFile(uintptr(fdesc), "acc"))
				if err != nil {
					ctl.lastErr.Store(err.Error())
					return
				}
				zc, err := (*zctx).DialWithOptions(shrToken, &ziti.DialOptions{ConnectTimeout: 30 * time.Second})
				if err != nil {
					ctl.lastErr.Store(err.Error())
					lc.Close()
					return
				}
				pumpPair(&countedZiti{Conn: zc, ctl: ctl}, lc)
			}(fd)
		}
	}()
	return jok(id)
}
func Stop(id string) string {
	if v, ok := tunnels.Load(id); ok {
		ctl := v.(*tunnelCtl)
		ctl.run.Store(false)
		if ctl.listener != nil {
			ctl.listener.Close()
		}
		return jok(nil)
	}
	return jerr(errors.New("no such tunnel"))
}
func Stats() string {
	out := []tunnelInfo{}
	tunnels.Range(func(_, v any) bool {
		out = append(out, v.(*tunnelCtl).snapshot())
		return true
	})
	b, _ := json.Marshal(out)
	return jok(string(b))
}
