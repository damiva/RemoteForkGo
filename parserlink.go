package main

import (
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

func init() {
	http.Handle("/parserlink", handler(parserlink))
}
func doCurl(url string, flw bool, hdr http.Header, pst string) (*http.Response, error) {
	var req *http.Request
	var cln *http.Client
	var err error
	if pst == "" {
		req, err = http.NewRequest("GET", url, nil)
	} else {
		req, err = http.NewRequest("POST", url, strings.NewReader(pst))
	}
	if err != nil {
		return nil, err
	}
	if hdr != nil && len(hdr) > 0 {
		req.Header = hdr
	}
	if flw {
		cln = &http.Client{}
	} else {
		cln = &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	}
	return cln.Do(req)
}
func parserlink(w http.ResponseWriter, r *http.Request) {
	s, ch, fr, hs, pb := r.URL.RawQuery, false, false, make(http.Header, 0), ""
	if r.Method == "POST" {
		b, e := ioutil.ReadAll(r.Body)
		check(e, false)
		s = string(b)
	}
	s, e := url.QueryUnescape(s)
	check(e, false)
	rq := strings.Split(s, "|")
	switch {
	case strings.HasPrefix(rq[0], "curlorig "):
		fallthrough
	case strings.HasPrefix(rq[0], "curl "):
		ch, fr = strings.Contains(rq[0], " -i"), strings.Contains(rq[0], " -L")
		for _, s := range regexp.MustCompile(`\s-H\s"(.*?)"`).FindAllStringSubmatch(rq[0], -1) {
			if h := strings.SplitN(s[1], ":", 2); len(h) > 1 {
				hs.Add(h[0], strings.TrimSpace(h[1]))
			}
		}
		if s := regexp.MustCompile(`\s(-d|--data)\s"(.*?)"`).FindStringSubmatch(rq[0]); s != nil && len(s) > 3 {
			pb = s[2]
			hs.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		rq[0] = regexp.MustCompile(`\s(-H|-d|--data)\s"(.*?)"`).ReplaceAllString(rq[0], "")
		if s := regexp.MustCompile(`\s"(.*?)"`).FindStringSubmatch(rq[0]); s != nil {
			rq[0] = s[1]
		}
	default:
		fr = true
	}
	rsp, e := doCurl(rq[0], fr, hs, pb)
	check(e, false)
	defer rsp.Body.Close()
	if ch {
		for k, v := range rsp.Header {
			w.Header()[k] = v
		}
		if len(rsp.TransferEncoding) > 0 {
			w.Header()["Transfer-Encoding"] = rsp.TransferEncoding
		}
		w.WriteHeader(rsp.StatusCode)
	}
	switch len(rq) {
	case 1:
		io.Copy(w, rsp.Body)
	case 2:
		var body string
		if b, e := ioutil.ReadAll(rsp.Body); e == nil {
			body = string(b)
		} else {
			panic(e)
		}
		if re := regexp.MustCompile(rq[1]).FindStringSubmatch(body); re != nil && len(re) > 0 {
			body = re[1]
			return
		}
		w.Write([]byte(body))
	default:
		var body string
		if b, e := ioutil.ReadAll(rsp.Body); e == nil {
			body = string(b)
		} else {
			panic(e)
		}
		if strings.Contains(rq[1], ".*?") {
			if re := regexp.MustCompile(rq[1] + "(.*?)" + rq[2]).FindStringSubmatch(body); re != nil && len(re) > 0 {
				body = re[1]
			}
		} else {
			if rq[1] != "" {
				if bs := strings.SplitN(body, rq[1], 2); len(bs) > 1 {
					if rq[2] != "" {
						body = strings.SplitN(body, rq[2], 2)[0]
					} else {
						body = bs[1]
					}
				} else {
					body = ""
				}
			}
		}
		w.Write([]byte(body))
	}
}
