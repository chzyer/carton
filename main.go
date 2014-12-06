package main

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var _ = log.Println
var host = "http://www.7330.com/"
var layout = "20060102"

//
var regexpImg = regexp.MustCompile(`pictureContent(?s:.+?)"([^"]+.(?:jpg|png))"`)

func get(url string) (content []byte, err error) {
	resp, err := http.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	content, err = ioutil.ReadAll(resp.Body)
	return
}

type content struct {
	Id   string
	Name string
}

func Recent(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	name := req.Form.Get("name")
	cookie, err := req.Cookie("current_" + name)
	if err == nil {
		http.Redirect(w, req, "/view?name="+name+"&"+cookie.Value, 302)
		return
	}
	cookies := req.Cookies()
	html := ""
	for _, cookie := range cookies {
		sn := cookie.Name
		if !strings.HasPrefix(sn, "current_") {
			continue
		}
		sn = cookie.Name[8:]
		html += `<a href="/view?name=` + sn + `&` + cookie.Value + `">` + sn + `</a><br>`
	}
	w.Write([]byte(html))
}

// 目录
func Content(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	_, force := req.Form["f"]
	etag := req.Header.Get("If-None-Match")
	if !force && etag == time.Now().Format(layout) {
		w.WriteHeader(304)
		return
	}
	name := req.Form.Get("name")
	cp := regexp.MustCompile(host + name + `/(\d+).shtml.*title="([^"]+)"`)
	ret, err := get(host + name)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	submatch := cp.FindAllSubmatch(ret, -1)
	rets := make([]content, len(submatch))
	for idx, sm := range submatch {
		rets[idx] = content{string(sm[1]), string(sm[2])}
	}
	ret, _ = json.Marshal(rets)
	header := w.Header()
	header.Set("Cache-Control", "publish, max-age=1")
	header.Set("Etag", time.Now().Format(layout))
	w.Write(ret)
}

func View(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	name := req.Form.Get("name")
	id := req.Form.Get("id")
	page := req.Form.Get("page")
	if page == "" {
		page = "1"
	}
	params := url.Values{
		"name": {name},
		"id":   {id},
		"page": {page},
	}
	p, _ := strconv.Atoi(page)
	next := strconv.Itoa(p + 1)
	prev := strconv.Itoa(p - 1)
	now := params.Encode()
	params["page"][0] = next
	np := params.Encode()
	params["page"][0] = prev
	pp := params.Encode()

	html := `<html onkeypress="keyPress">
<head>
<meta name="viewport" content="height=device-height">
<meta charset="utf-8" />
</head>
<body>
<div>
<img src="/img?` + now + `" id="pic" onclick="next()"/>
<img style="display:none" src="" />
</div>
<script src="/jquery.js" type="text/javascript"></script>
<script>
var name = "` + name + `";
var current = ` + page + `;
var id = ` + id + `;
var u = [
	"/view?` + pp + `",
	"/view?` + np + `"
];
var loading = true
var notNext = false
$.get("/img?` + np + `")
var right = function(){
	if ($(this).width() > document.body.clientWidth*1.2) {
		$("body").animate({scrollLeft: 1000000}, 1)
	}
}
$(document).ready(function() {
	right()
	$("#pic").load(right)
})
function next() {
	if ($("body").scrollLeft() != 0) {
		$("body").animate({scrollLeft: 0, scrollTop:0}, 100)
		return
	}
	if (notNext) {
		alert("已经是最后一页")
		return
	}
	location.href = u[1]
}
$.get("/info" + location.search, function(e) {
	var page = e;
	$.get("/content?name=" + name + "&id=" + id, function(e) {
		var obj = JSON.parse(e)
		if (current >= page) {
			for (var i in obj) {
				if (obj[i].Id == id) {
					if (i == 0) {
						notNext = true
						break
					}
					u[1] = "/view?name=" + name + "&id=" + obj[i-1].Id
				}
			}
			loading = false
		} else if (current == 1) {
			for (var i in obj) {
				if (obj[i].Id == id) {
					var search = "name=" + name + "&id=" + obj[i-0+1].Id
					$.get("/info?" + search, function(e) {
						u[0] = "/view?" + search + "&page=" + e
						loading = false
					})
				}
			}
		} else {
			loading = false
		}
	})
})
function keyPress(e) {
	var pK = e? e.which: window.event.keyCode;
	var i = -1
	var height = 40
	if (loading) {
		return
	}
	if (pK == 32) {
		i = 1
		next()
		return false
	} else if (pK == 59) {
		i = 0
	} else if (pK == 106) { // j
		$(window).scrollTop($(window).scrollTop()+height)
	} else if (pK == 107) { // k
		$(window).scrollTop($(window).scrollTop()-height)
	} else if (pK == 104) { // h
		$(window).scrollLeft($(window).scrollLeft()-height)
	} else if (pK == 108) { // l
		$(window).scrollLeft($(window).scrollLeft()+height)
	}
	if (i == -1) {
		return true
	}
	location.href = u[i]
	return false
}
document.onkeypress = keyPress
</script>
</div>
</html>`
	http.SetCookie(w, &http.Cookie{Name: "current_" + name, Value: url.Values{"id": {id}, "page": {page}}.Encode()})
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

var etag = "Hello"

func getEtag(req *http.Request) string {
	return req.Header.Get("If-None-Match")
}

func Img(w http.ResponseWriter, req *http.Request) {
	if getEtag(req) == etag {
		w.WriteHeader(304)
		return
	}
	req.ParseForm()
	name := req.Form.Get("name")
	id := req.Form.Get("id")
	page := req.Form.Get("page")
	if page != "" && page != "1" {
		page = "_" + page
	} else {
		page = ""
	}
	url_ := host + name + "/" + id + page + ".shtml"
	body, err := get(url_)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	submatch := regexpImg.FindSubmatch(body)
	if len(submatch) < 1 {
		http.Error(w, "img not found"+url_, 500)
		return
	}
	url_ = string(submatch[1])

	u, err := url.Parse(url_)
	if err != nil {
		println(err.Error())
		return
	}
	resp_, err := http.Get(url_)
	if err != nil {
		println(err.Error())
		return
	}
	resp_.Body.Close()
	if resp_.Header.Get("Content-Type") == "text/html" {
		url_ = u.Query().Get("img")
	}

	println(url_)

	resp, err := http.Get(url_)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Cache-Control", "public, max-age=31536000")
	w.Header().Set("Etag", etag)
	io.Copy(w, resp.Body)
}

func Page(w http.ResponseWriter, req *http.Request) {

}

func jquery(w http.ResponseWriter, req *http.Request) {
	f, err := os.Open("jquery.js")
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}
	defer f.Close()
	io.Copy(w, f)
}

// 单集漫画信息
func Info(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	if getEtag(req) == etag {
		w.WriteHeader(304)
		return
	}
	name := req.Form.Get("name")
	id := req.Form.Get("id")
	url_ := host + name + "/" + id + ".shtml"
	ret, err := get(url_)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	ret, _ = json.Marshal(strings.Count(string(ret), "<option") / 2)
	w.Header().Set("Etag", etag)
	w.Write(ret)
}

func Index(w http.ResponseWriter, req *http.Request) {
	http.Redirect(w, req, "/recent", 302)
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", Index)
	mux.HandleFunc("/page", Page)
	mux.HandleFunc("/img", Img)
	mux.HandleFunc("/view", View)
	mux.HandleFunc("/content", Content)
	mux.HandleFunc("/jquery.js", jquery)
	mux.HandleFunc("/info", Info)
	mux.HandleFunc("/recent", Recent)
	println("please open http://localhost:8900/ to see carton")
	err := http.ListenAndServe(":8900", mux)
	if err != nil {
		panic(err)
	}
}
