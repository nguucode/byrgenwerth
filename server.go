package main

// Local web page for OBS "Browser Source": shows the counter with the same
// style as the on-screen overlay, on a transparent background, and updates
// itself. Bound to 127.0.0.1 only.

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

type OverlayState struct {
	Text         string `json:"text"`
	FontFamily   string `json:"font"`
	FontSize     int    `json:"size"`
	Bold         bool   `json:"bold"`
	Color        string `json:"color"`
	Outline      bool   `json:"outline"`
	OutlineWidth int    `json:"outlineWidth"`
	OutlineColor string `json:"outlineColor"`
	Version      int64  `json:"v"`
}

func StateFromConfig(c Config, text string) OverlayState {
	return OverlayState{
		Text: text, FontFamily: c.FontFamily, FontSize: c.OverlayFontSize, Bold: c.FontBold,
		Color: c.TextColor, Outline: c.Outline && c.OutlineWidth > 0, OutlineWidth: c.OutlineWidth, OutlineColor: c.OutlineColor,
	}
}

type OverlayServer struct {
	mu     sync.Mutex
	state  OverlayState
	splits SplitsState
	ln     net.Listener
	srv    *http.Server
}

// StartOverlayServer listens on 127.0.0.1, trying port..port+20.
func StartOverlayServer(port int) (*OverlayServer, error) {
	s := &OverlayServer{}
	var ln net.Listener
	var err error
	for p := port; p <= port+20; p++ {
		ln, err = net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, err
	}
	s.ln = ln
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handlePage)
	mux.HandleFunc("/state", s.handleState)
	mux.HandleFunc("/bosses", s.handleBossPage)
	mux.HandleFunc("/bosses/state", s.handleBossState)
	s.srv = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go s.srv.Serve(ln)
	return s, nil
}

func (s *OverlayServer) URL() string {
	if s == nil {
		return ""
	}
	return "http://" + s.ln.Addr().String() + "/"
}

func (s *OverlayServer) Set(st OverlayState) {
	if s == nil {
		return
	}
	s.mu.Lock()
	st.Version = s.state.Version + 1
	s.state = st
	s.mu.Unlock()
}

func (s *OverlayServer) Close() {
	if s != nil {
		s.srv.Close()
	}
}

func (s *OverlayServer) handleState(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	st := s.state
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(st)
}

func (s *OverlayServer) handlePage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	fmt.Fprint(w, overlayPage)
}

// The outline is a stroked copy behind the fill, so it grows outward and never
// eats into the letters (same look as the native overlay).
const overlayPage = `<!doctype html>
<html><head><meta charset="utf-8"><title>Death Counter</title>
<style>
html,body{margin:0;background:transparent;overflow:hidden}
#w{position:relative;display:inline-block;white-space:pre;padding:8px;line-height:1.2}
#w span{display:block}
#f{position:relative;z-index:1}
#o{position:absolute;z-index:0;left:8px;top:8px;-webkit-text-fill-color:transparent}
</style></head>
<body><div id="w"><span id="o"></span><span id="f"></span></div>
<script>
var v=-1,w=document.getElementById('w'),o=document.getElementById('o'),f=document.getElementById('f');
function apply(s){
  w.style.fontFamily='"'+s.font.replace(/"/g,'')+'", "Segoe UI", sans-serif';
  w.style.fontSize=s.size+'px';
  w.style.fontWeight=s.bold?'700':'400';
  f.style.color=s.color;
  o.textContent=f.textContent=s.text;
  if(s.outline){o.style.display='block';o.style.webkitTextStroke=(s.outlineWidth*2)+'px '+s.outlineColor;w.style.padding=(8+s.outlineWidth)+'px';o.style.left=o.style.top=(8+s.outlineWidth)+'px';}
  else{o.style.display='none';w.style.padding='8px';}
}
function tick(){
  fetch('/state',{cache:'no-store'}).then(function(r){return r.json()}).then(function(s){if(s.v!==v){v=s.v;apply(s)}}).catch(function(){}).then(function(){setTimeout(tick,300)});
}
tick();
</script></body></html>`

func (s *OverlayServer) SetSplits(st SplitsState) {
	if s == nil {
		return
	}
	s.mu.Lock()
	st.Version = s.splits.Version + 1
	s.splits = st
	s.mu.Unlock()
}

func (s *OverlayServer) BossURL() string {
	if s == nil {
		return ""
	}
	return s.URL() + "bosses"
}

func (s *OverlayServer) handleBossState(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	st := s.splits
	s.mu.Unlock()
	if st.Rows == nil {
		st.Rows = []SplitRow{}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(st)
}

func (s *OverlayServer) handleBossPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	fmt.Fprint(w, bossPage)
}

// LiveSplit-like splits: dark rows, rank medal colors, right-aligned tabular
// counts, the boss being fought highlighted in blue.
const bossPage = `<!doctype html>
<html><head><meta charset="utf-8"><title>Top bosses</title>
<style>
html,body{margin:0;background:transparent;overflow:hidden}
#p{display:inline-block;min-width:22em;background:linear-gradient(#1b1b20,#0b0b0d);color:#fff;
  font-family:var(--f),"Segoe UI",sans-serif;font-size:var(--s);border:1px solid #2b2b33}
.h{display:flex;justify-content:space-between;padding:.35em .6em;font-size:.72em;letter-spacing:.08em;
  text-transform:uppercase;color:#9a9aa6;background:linear-gradient(#26262d,#16161a);border-bottom:1px solid #2b2b33}
.r{display:flex;align-items:center;gap:.55em;padding:.32em .6em;border-bottom:1px solid #1f1f25;
  background:linear-gradient(#141418,#0e0e11)}
.r.a{background:linear-gradient(#2f62d6,#1a3c94);border-bottom-color:#3a6fe6}
.k{width:1.4em;text-align:right;font-weight:700;color:#8b8b96;font-variant-numeric:tabular-nums}
.r1 .k{color:#ffd23f}.r2 .k{color:#d7d7e0}.r3 .k{color:#d9894a}
.n{flex:1;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;max-width:18em}
.c{font-weight:700;font-variant-numeric:tabular-nums;min-width:2.2em;text-align:right}
.r1 .c{color:#ffd23f}.a .c{color:#fff}
.sep{height:3px;background:#2b2b33}
.f{display:flex;justify-content:space-between;padding:.4em .6em;font-size:.85em;color:#c8c8d0;background:#101013}
.f b{color:#ff4d4d;font-variant-numeric:tabular-nums}
.e{padding:.6em;color:#8b8b96;font-size:.85em}
</style></head>
<body><div id="p"></div>
<script>
var v=-1,p=document.getElementById('p');
function esc(t){return String(t).replace(/[&<>"]/g,function(c){return{'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;'}[c]})}
function draw(s){
  p.style.setProperty('--f','"'+(s.font||'Segoe UI').replace(/"/g,'')+'"');
  p.style.setProperty('--s',(s.size||18)+'px');
  var h='<div class="h"><span>'+esc(s.title||'Top boss')+'</span><span>Deaths</span></div>';
  if(!s.rows.length){h+='<div class="e">Chưa có dữ liệu boss</div>'}
  s.rows.forEach(function(r,i){
    if(r.rank===0||r.rank>3){if(i>0)h+='<div class="sep"></div>'}
    var cls='r'+(r.rank>=1&&r.rank<=3?' r'+r.rank:'')+(r.active?' a':'');
    h+='<div class="'+cls+'"><span class="k">'+(r.rank?r.rank:'▶')+'</span><span class="n">'+esc(r.name)+'</span><span class="c">'+r.deaths+'</span></div>';
  });
  h+='<div class="f"><span>Tổng số lần chết</span><b>'+s.total+'</b></div>';
  p.innerHTML=h;
}
function tick(){fetch('/bosses/state',{cache:'no-store'}).then(function(r){return r.json()}).then(function(s){if(s.v!==v){v=s.v;draw(s)}}).catch(function(){}).then(function(){setTimeout(tick,400)})}
tick();
</script></body></html>`
