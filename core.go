package core

import (
	"core/proxy"
	"encoding/json"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/xujiajun/nutsdb"
)

type Core struct {
	Debug    bool
	listener net.Listener
	Server   *http.Server
	proxy    proxy.Proxy
}

func New(conf map[string]interface{}) *Core {
	c := &Core{
		Server: new(http.Server),
		proxy:  proxy.Proxy{},
	}
	if debug, ok := conf["debug"].(bool); ok {
		c.Debug = debug
		c.Server.ErrorLog = log.New(os.Stdout, "DEBUG", log.Ldate|log.Ltime)
	}
	opt := nutsdb.DefaultOptions
	opt.Dir = "db"
	db, err := nutsdb.Open(opt)
	if err == nil {
		c.proxy.DB = db
		c.loadAPIs()
	}

	if addr, ok := conf["listen"]; ok {
		ln, err := Listen(addr)
		if err != nil {
			panic(err)
		}
		c.listener = ln
	}
	if ReadTimeout, ok := conf["ReadTimeout"].(int); ok {
		c.Server.ReadTimeout = time.Duration(time.Second * time.Duration(ReadTimeout))
	}
	if ReadHeaderTimeout, ok := conf["ReadHeaderTimeout"].(int); ok {
		c.Server.ReadHeaderTimeout = time.Duration(time.Second * time.Duration(ReadHeaderTimeout))
	}
	if WriteTimeout, ok := conf["WriteTimeout"].(int); ok {
		c.Server.WriteTimeout = time.Duration(time.Second * time.Duration(WriteTimeout))
	}
	if IdleTimeout, ok := conf["IdleTimeout"].(int); ok {
		c.Server.IdleTimeout = time.Duration(time.Second * time.Duration(IdleTimeout))
	}
	if MaxHeaderBytes, ok := conf["MaxHeaderBytes"].(int); ok {
		c.Server.MaxHeaderBytes = MaxHeaderBytes
	}
	c.Server.Handler = c
	return c
}

func (c *Core) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Server", "mgw")
	defer func() {
		if rc := recover(); rc != nil {
			log.Printf("[PANIC] %v", rc)
			ErrorFunc(w, r, http.StatusInternalServerError)
		}
	}()

	if strings.HasPrefix(r.URL.Path, "/mgw") {
		c.admin(w, r)
		return
	}

	var duration time.Duration
	start := time.Now()
	status, err := c.proxy.ServeHTTP(w, r)
	duration = time.Since(start)

	if status >= 400 {
		w.Header().Set("duration", duration.String())
		WriteTextResp(w, status, err.Error())
	}
}

func (c *Core) Start() error {
	err := c.Server.Serve(c.listener)
	c.saveAPIs()
	c.proxy.DB.Close()
	return err
}

func (c *Core) admin(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/mgw/sign":
		c.sign(w, r)
	case "/mgw/delete":
		c.del(w, r)
	case "/mgw/save":
		c.saveAPIs()
		WriteTextResp(w, http.StatusOK, "ok")
	case "/mgw/load":
		c.loadAPIs()
		WriteTextResp(w, http.StatusOK, "ok")
	case "/mgw":
		c.analysis(w, r)
	}
}

// 列出API信息
func (c *Core) analysis(w http.ResponseWriter, r *http.Request) {
	apis := make([]map[string]interface{}, 0)
	c.proxy.APIs.Range(func(key, value interface{}) bool {
		k := key.(string)
		v := value.(proxy.Upstream)
		hosts := make([]interface{}, 0)
		for _, host := range v.GetHosts() {
			hosts = append(hosts, host.Info())
		}
		apis = append(apis, map[string]interface{}{
			k: hosts,
		})
		return true
	})
	// buf, err := json.Marshal(apis)
	// if err != nil {
	// 	fmt.Println(err)
	// }
	tmpl, err := template.New("info").Parse(tpl)
	if err != nil {
		WriteTextResp(w, http.StatusInternalServerError, "Build failed")
		return
	}
	err = tmpl.Execute(w, apis)
	if err != nil {
		WriteTextResp(w, http.StatusInternalServerError, err.Error())
	}
}

var tpl = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta http-equiv="X-UA-Compatible" content="IE=edge">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>mgw info</title>
    <style>
        .box{
            margin-left: 24pt;
            padding: 20pt 0 0 24pt;
        }
		table{
			width:100%;
		}
		td {
			border-bottom: 1pt solid #ebeef5;
		}
    </style>
</head>
<body>
    {{- range $item := .}}
    <div>
        {{- range $k,$v := $item}}
        <div class="box">
            <div>{{ $k }}
				<a href="/mgw/delete?api={{ $k | urlquery }}">Del</a>
			</div>
            <table>
            {{- range $val := $v }}
                <tr>
                    <td>{{ $val.name }}</td>
                    <td>{{ $val.host }}</td>
                    <td>{{ $val.available }}</td>
					<td><a href="/mgw/delete?host={{ $val.host | urlquery }}">Del</a></td>
                </tr>
			{{- end }}
            </table>
        </div>
        {{- end }}
    </div>
    {{- end }}
</body>
</html>`

func (c *Core) del(w http.ResponseWriter, r *http.Request) {
	if api, ok := r.URL.Query()["api"]; ok {
		c.delAPI(w, r, api[0])
		return
	}
	if host, ok := r.URL.Query()["host"]; ok {
		c.delHost(w, r, host[0])
	}
}

func (c *Core) delAPI(w http.ResponseWriter, r *http.Request, v string) {
	c.proxy.APIs.Delete(v)
	c.saveAPIs()
	http.Redirect(w, r, "/mgw", http.StatusFound)
}
func (c *Core) delHost(w http.ResponseWriter, r *http.Request, v string) {
	c.proxy.APIs.Range(func(key, value interface{}) bool {
		u := value.(proxy.Upstream)
		for range u.GetHosts() {
			if u.DelHost(v) == 0 {
				c.proxy.APIs.Delete(key.(string))
			}
		}
		return true
	})
	c.saveAPIs()
	http.Redirect(w, r, "/mgw", http.StatusFound)
}

// sign 注册API
func (c *Core) sign(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost { // 上传数据
		form := make(map[string]interface{})
		if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
			log.Println(err)
			WriteTextResp(w, http.StatusBadRequest, err.Error())
			return
		}
		var healthCheck proxy.HealthCheck

		name := ""
		if n, ok := form["name"].(string); ok {
			name = n
			delete(form, "name")
		}

		if setCheck, ok := form["check"]; ok {
			chk := setCheck.(map[string]interface{})
			method := "GET"
			mhd, ok := chk["method"].(string)
			if ok {
				method = strings.ToUpper(mhd)
			}
			path := ""
			ptv, ok := chk["path"].(string)
			if ok {
				path = ptv
			}
			interval := 60
			timeout := 60
			itv, ok := chk["interval"].(float64)
			if ok {
				interval = int(itv)
			}
			tmo, ok := chk["timeout"].(float64)
			if ok {
				timeout = int(tmo)
			}
			healthCheck = proxy.HealthCheck{
				Method:   method,
				Path:     path,
				Interval: interval,
				Timeout:  timeout,
			}
			delete(form, "check")
		}
		for srv, apis := range form {
			v := apis.([]interface{})
			for _, api := range v {
				c.signApi(srv, name, api.(string), healthCheck)
			}
		}
		c.saveAPIs()
		WriteTextResp(w, http.StatusOK, "ok")
		return
	}
	w.Write([]byte(`// mgw.addr this server online address
// microservice.ip you micro service address
POST http://{mgw.addr}/mgw/sign

// check: health check  Optional
//   method: get | post | options | .... all request method
//   path: /check 
//   interval: int second
//   timeout:  int second
// api address support * 
Payload:
{
	"check": {
		"method": "get",
		"path": "/check",
		"interval": 30,
		"timeout": 50,
	},
  "http://microservice.ip:40000": [ 
	"/api/users/auth",
	"/api/users/authorize",
	"/api/users/department",
	"/api/users/department/*",
	"/api/users/department/sync",
	"/api/users/dept/us/*",
	"/api/users/sync/*"
  ]
}`))
}

func (c *Core) signApi(srv, name, api string, healthCheck proxy.HealthCheck) error {
	done := false

	if name == "" {
		name = srv
	}
	interval := time.Second * time.Duration(healthCheck.Interval)
	timeout := time.Second * time.Duration(healthCheck.Timeout)
	c.proxy.APIs.Range(func(key, value interface{}) bool {
		u := value.(proxy.Upstream)
		k := key.(string)
		if strings.Compare(k, api) == 0 {
			done = true
			for _, host := range u.GetHosts() {
				if host.Name == srv {
					return false
				}
			}
			u.AddHost(srv, name)
			if healthCheck.Path != "" {
				u.SetHealthCheck(healthCheck.Path, healthCheck.Method, interval, timeout)
			}
			return false
		}
		return true
	})
	if done {
		return nil
	}
	u, err := proxy.NewUpstream(srv, name)
	if err == nil {
		if healthCheck.Path != "" {
			u.SetHealthCheck(healthCheck.Path, healthCheck.Method, interval, timeout)
		}
		c.proxy.APIs.Store(api, u)
	}
	return err
}

func (c *Core) saveAPIs() {
	apis := make(apiData)
	c.proxy.APIs.Range(func(key, value interface{}) bool {
		k := key.(string)
		v := value.(proxy.Upstream)
		hosts := make([]map[string]string, 0)
		for _, host := range v.GetHosts() {
			hc := v.GetHealthCheck()
			hosts = append(hosts, map[string]string{
				"h": host.Name,
				"s": host.Server,
				"m": hc.Method,
				"p": hc.Path,
				"i": strconv.Itoa(hc.Interval),
				"t": strconv.Itoa(hc.Timeout),
			})
		}
		apis[k] = hosts
		return true
	})
	data, err := json.Marshal(apis)
	if err == nil {
		func() {
			if err := c.proxy.DB.Update(func(tx *nutsdb.Tx) error {
				return tx.Put("apis", []byte("apis"), data, 0)
			}); err != nil {
				log.Println(err)
			}
		}()
	}
}

func (c *Core) loadAPIs() {
	c.proxy.DB.View(func(tx *nutsdb.Tx) error {
		e, err := tx.Get(apiTable, apiKey)
		if err != nil {
			return err
		}
		apis := make(apiData)
		if err := json.Unmarshal(e.Value, &apis); err != nil {
			return err
		}
		for k, v := range apis {
			for _, h := range v {
				iv, err := strconv.Atoi(h["i"])
				if err != nil {
					iv = 30
				}
				to, err := strconv.Atoi(h["t"])
				if err != nil {
					to = 30
				}
				hc := proxy.HealthCheck{
					Method:   h["m"],
					Path:     h["p"],
					Interval: iv,
					Timeout:  to,
				}
				c.signApi(h["h"], h["s"], k, hc)
			}
		}
		return nil
	})
}

var (
	apiTable = "apis"
	apiKey   = []byte("apis")
)

type apiData = map[string][]map[string]string
