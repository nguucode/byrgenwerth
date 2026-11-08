package main

// Local web page for OBS "Browser Source": shows the counter and its label with
// the same style as the on-screen overlay, on a transparent background, and
// updates itself. Bound to 127.0.0.1 only.

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type OverlayState struct {
	Count        string `json:"count"`
	Label        string `json:"label"`    // word, "" when none
	LabelImg     string `json:"labelImg"` // picture URL, "" when none
	LabelSize    int    `json:"labelSize"`
	Corner       string `json:"corner"`
	FontFamily   string `json:"font"`
	FontSize     int    `json:"size"`
	Bold         bool   `json:"bold"`
	Color        string `json:"color"`
	Outline      bool   `json:"outline"`
	OutlineWidth int    `json:"outlineWidth"`
	OutlineColor string `json:"outlineColor"`
	Version      int64  `json:"v"`
}

// StateFromConfig builds the page state; imgVersion changes the picture URL so
// OBS does not keep showing an old picture from its cache.
func StateFromConfig(c Config, game string, count int, imgVersion int) OverlayState {
	st := OverlayState{
		Count: fmt.Sprint(count), Label: LabelTextFor(c, game), LabelSize: c.LabelSize, Corner: c.LabelCorner,
		FontFamily: c.FontFamily, FontSize: c.OverlayFontSize, Bold: c.FontBold,
		Color: c.TextColor, Outline: c.Outline && c.OutlineWidth > 0, OutlineWidth: c.OutlineWidth, OutlineColor: c.OutlineColor,
	}
	if c.LabelMode == LabelModeImage && c.LabelImage != "" {
		st.LabelImg = fmt.Sprintf("/label?v=%d", imgVersion)
	}
	return st
}

type OverlayServer struct {
	mu       sync.Mutex
	state    OverlayState
	labelImg string // file served at /label
	ln       net.Listener
	srv      *http.Server
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
	mux.HandleFunc("/label", s.handleLabel)
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

// SetLabelImage chooses the picture file served at /label ("" = none).
func (s *OverlayServer) SetLabelImage(path string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.labelImg = path
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

func (s *OverlayServer) handleLabel(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	path := s.labelImg
	s.mu.Unlock()
	b, err := os.ReadFile(path)
	if path == "" || err != nil {
		http.NotFound(w, r)
		return
	}
	typ := "image/png"
	if strings.EqualFold(filepath.Ext(path), ".svg") {
		typ = "image/svg+xml"
	}
	w.Header().Set("Content-Type", typ)
	w.Header().Set("Cache-Control", "no-store")
	w.Write(b)
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

// Each text block is a stroked copy behind the fill, so the outline grows
// outward and never eats into the letters (same look as the native overlay).
// The label sits above (tl, tr) or below (bl, br) the count, on its left or
// right edge.
const overlayPage = `<!doctype html>
<html><head><meta charset="utf-8"><title>Byrgenwerth</title>
<style>
html,body{margin:0;background:transparent;overflow:hidden}
#w{display:inline-flex;flex-direction:column;padding:8px;line-height:1.2}
.t{position:relative;display:inline-block;white-space:pre}
.t .f{position:relative;z-index:1}
.t .o{position:absolute;z-index:0;left:0;top:0;-webkit-text-fill-color:transparent}
#lab img{display:block}
</style></head>
<body><div id="w"><div id="lab"></div><div id="cnt"></div></div>
<script>
var v=-1,img='',w=document.getElementById('w'),lab=document.getElementById('lab'),cnt=document.getElementById('cnt');
function text(el,s,t,size){
  el.innerHTML='<span class="t"><span class="o"></span><span class="f"></span></span>';
  var o=el.querySelector('.o'),f=el.querySelector('.f'),b=el.firstChild;
  b.style.fontSize=size+'px';
  f.style.color=s.color;
  o.textContent=f.textContent=t;
  if(s.outline){o.style.display='block';o.style.webkitTextStroke=(s.outlineWidth*2)+'px '+s.outlineColor;b.style.margin=s.outlineWidth+'px';}
  else{o.style.display='none';}
}
function apply(s){
  w.style.fontFamily='"'+s.font.replace(/"/g,'')+'", "Segoe UI", sans-serif';
  w.style.fontWeight=s.bold?'700':'400';
  var bottom=s.corner==='bl'||s.corner==='br',right=s.corner==='tr'||s.corner==='br';
  w.style.flexDirection=bottom?'column-reverse':'column';
  w.style.alignItems=right?'flex-end':'flex-start';
  text(cnt,s,s.count,s.size);
  if(s.labelImg){
    if(img!==s.labelImg||!lab.querySelector('img')){lab.innerHTML='<img alt="">';lab.firstChild.src=s.labelImg;img=s.labelImg;}
    lab.firstChild.style.height=s.labelSize+'px';
  }else if(s.label){text(lab,s,s.label,s.labelSize);img='';}
  else{lab.innerHTML='';img='';}
}
function tick(){
  fetch('/state',{cache:'no-store'}).then(function(r){return r.json()}).then(function(s){if(s.v!==v){v=s.v;apply(s)}}).catch(function(){}).then(function(){setTimeout(tick,300)});
}
tick();
</script></body></html>`
