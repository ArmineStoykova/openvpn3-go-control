package main

import (
 "context"
 "encoding/json"
 "errors"
 "fmt"
 "log"
 "net/http"
 "os"
 "os/exec"
 "path/filepath"
 "regexp"
 "strings"
 "sync"
 "time"
)

const defaultConfig = "/home/armine/Downloads/armine.stoykova.ovpn"
const address = "127.0.0.1:8765"

var (
 config = getenv("OPENVPN_CONFIG", defaultConfig)
 mu sync.Mutex
 pathRE = regexp.MustCompile(`/net/openvpn/v3/sessions/[A-Za-z0-9_-]+`)
)

type reply struct { State string `json:"state"`; Message string `json:"message"`; Details string `json:"details,omitempty"`; Sessions []string `json:"sessions,omitempty"` }
func getenv(k, fallback string) string { if v:=os.Getenv(k);v!="" {return v};return fallback }
func command(timeout time.Duration, args ...string)(string,error){
 ctx,cancel:=context.WithTimeout(context.Background(),timeout);defer cancel()
 cmd:=exec.CommandContext(ctx,"openvpn3",args...)
 out,err:=cmd.CombinedOutput()
 s:=strings.TrimSpace(string(out))
 if ctx.Err()==context.DeadlineExceeded {return s,fmt.Errorf("command timed out after %s",timeout)}
 if err!=nil {return s,fmt.Errorf("%w",err)}
 return s,nil
}

type session struct { path, block string }
func sessions()(string,[]session,error){
 out,err:=command(12*time.Second,"sessions-list")
 if err!=nil{return out,nil,err}
 lines:=strings.Split(out,"\n")
 var found []session
 // Each session starts at a Path: line; attach surrounding lines until the next Path.
 for i,line:=range lines {
  if !strings.Contains(line,"Path:"){continue}
  p:=pathRE.FindString(line);if p==""{continue}
  end:=len(lines);for j:=i+1;j<len(lines);j++ {if strings.Contains(lines[j],"Path:") && pathRE.MatchString(lines[j]){end=j;break}}
  found=append(found,session{p,strings.Join(lines[i:end],"\n")})
 }
 return out,found,nil
}
func ownSessions()(string,[]session,error){
 out,all,err:=sessions();if err!=nil{return out,nil,err}
 var own []session
 // Match the full configuration path in OpenVPN 3's reported session details.
 for _,s:=range all {if strings.Contains(s.block,config){own=append(own,s)}}
 return out,own,nil
}
func status()reply{
 out,own,err:=ownSessions()
 if err!=nil{return reply{"error","Cannot query OpenVPN 3 sessions",out+"\n"+err.Error(),nil}}
 if len(own)==0{return reply{"disconnected","No session for the configured VPN file",out,nil}}
 var paths []string; connected:=false; pending:=false
 for _,s:=range own {
  paths=append(paths,s.path)
  low:=strings.ToLower(s.block)
  if strings.Contains(low,"connected") && !strings.Contains(low,"disconnected") {connected=true}
  if strings.Contains(low,"auth") || strings.Contains(low,"connecting") {pending=true}
 }
 if connected{return reply{"connected","OpenVPN 3 reports a connected session",out,paths}}
 if pending{return reply{"pending","VPN session exists; authentication or connection may still be in progress",out,paths}}
 return reply{"pending","VPN session exists but is not confirmed connected",out,paths}
}
func write(w http.ResponseWriter,code int,r reply){w.Header().Set("Content-Type","application/json; charset=utf-8");w.Header().Set("Cache-Control","no-store");w.WriteHeader(code);_ =json.NewEncoder(w).Encode(r)}
func validOrigin(r *http.Request)bool{
 origin:=r.Header.Get("Origin")
 if origin!="" && origin!="http://"+address{return false}
 // Protect against DNS rebinding and cross-origin browser requests.
 return r.Host==address && r.Header.Get("Sec-Fetch-Site")!="cross-site"
}
func main(){
 if _,err:=exec.LookPath("openvpn3");err!=nil{log.Fatal("openvpn3 executable not found in PATH: ",err)}
 mux:=http.NewServeMux()
 mux.HandleFunc("GET /",func(w http.ResponseWriter,r *http.Request){if r.URL.Path!="/"{http.NotFound(w,r);return};w.Header().Set("Content-Type","text/html; charset=utf-8");w.Header().Set("Cache-Control","no-store");fmt.Fprint(w,page)})
 mux.HandleFunc("GET /api/status",func(w http.ResponseWriter,r *http.Request){if !validOrigin(r){http.Error(w,"Forbidden",403);return};mu.Lock();defer mu.Unlock();write(w,200,status())})
 mux.HandleFunc("POST /api/on",func(w http.ResponseWriter,r *http.Request){
  if !validOrigin(r){http.Error(w,"Forbidden",403);return}
  mu.Lock();defer mu.Unlock()
  if fi,err:=os.Stat(config);err!=nil||fi.IsDir(){write(w,400,reply{"error","VPN configuration file is missing or invalid",fmt.Sprint(err),nil});return}
  _,own,err:=ownSessions();if err!=nil{write(w,500,reply{"error","Could not check existing sessions",err.Error(),nil});return}
  if len(own)>0{write(w,200,status());return}
  output,err:=command(35*time.Second,"session-start","--config",config)
  if err!=nil{write(w,500,reply{"error","Failed to start VPN",output+"\n"+err.Error(),nil});return}
  s:=status();s.Details=strings.TrimSpace(output+"\n"+s.Details);write(w,200,s)
 })
 mux.HandleFunc("POST /api/off",func(w http.ResponseWriter,r *http.Request){
  if !validOrigin(r){http.Error(w,"Forbidden",403);return}
  mu.Lock();defer mu.Unlock()
  _,own,err:=ownSessions();if err!=nil{write(w,500,reply{"error","Could not check existing sessions",err.Error(),nil});return}
  if len(own)==0{write(w,200,reply{"disconnected","No matching VPN session to disconnect","",nil});return}
  var outputs []string;var errs []error
  for _,s:=range own {out,e:=command(20*time.Second,"session-manage","--session-path",s.path,"--disconnect");outputs=append(outputs,s.path+": "+out);if e!=nil{errs=append(errs,fmt.Errorf("%s: %w",s.path,e))}}
  result:=status();result.Details=strings.Join(outputs,"\n")+"\n"+result.Details
  if len(errs)>0{result.State="error";result.Message="One or more disconnect commands failed";result.Details+="\n"+errors.Join(errs...).Error();write(w,500,result);return}
  write(w,200,result)
 })
 log.Printf("VPN control: http://%s | config: %s",address,filepath.Clean(config))
 server:=&http.Server{Addr:address,Handler:mux,ReadHeaderTimeout:5*time.Second}
 log.Fatal(server.ListenAndServe())
}

const page = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>OpenVPN 3 Control</title><style>
:root{color-scheme:dark;font-family:system-ui,Arial,sans-serif}body{background:#101827;color:#e8eef9;max-width:650px;margin:60px auto;padding:0 20px}main{background:#1c293d;border:1px solid #35435b;border-radius:18px;padding:28px}h1{margin-top:0}.status{font-weight:700;font-size:1.2rem;margin:20px 0}.actions{display:flex;gap:12px;flex-wrap:wrap}button{border:0;border-radius:10px;padding:13px 22px;font-size:1rem;font-weight:700;cursor:pointer}button:disabled{opacity:.5;cursor:wait}.on{background:#48c78e;color:#092317}.off{background:#ff7272;color:#290b0b}.refresh{background:#92baff;color:#102443}pre{white-space:pre-wrap;overflow-wrap:anywhere;background:#101827;padding:16px;border-radius:10px;max-height:330px;overflow:auto}small{color:#aebdd2}</style></head><body><main><h1>OpenVPN 3 Control</h1><small>Local Ubuntu control panel · 127.0.0.1 only</small><div class="status" id="status">Checking…</div><div class="actions"><button class="on" id="on">VPN-on</button><button class="off" id="off">VPN-off</button><button class="refresh" id="refresh">VPN-status</button></div><h3>OpenVPN output</h3><pre id="details">Loading…</pre><small>Some VPN profiles require browser-based authentication after VPN-on.</small></main><script>
const statusEl=document.getElementById('status'),details=document.getElementById('details'),buttons=[...document.querySelectorAll('button')];let busy=false;
async function request(action){if(busy)return;busy=true;buttons.forEach(b=>b.disabled=true);statusEl.textContent='Working…';try{const response=await fetch('/api/'+action,{method:action==='status'?'GET':'POST',cache:'no-store'});const result=await response.json();statusEl.textContent=result.state.toUpperCase()+' — '+result.message;details.textContent=result.details||result.message;if(!response.ok)statusEl.textContent='ERROR — '+result.message;}catch(e){statusEl.textContent='ERROR — Cannot contact local Go server';details.textContent=String(e)}finally{busy=false;buttons.forEach(b=>b.disabled=false)}}
document.getElementById('on').onclick=()=>request('on');document.getElementById('off').onclick=()=>request('off');document.getElementById('refresh').onclick=()=>request('status');request('status');setInterval(()=>{if(!busy)request('status')},10000);
</script></body></html>`
