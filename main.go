package main

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type Config struct {
	URL        string
	UserAgent  string
	Cookie     string
	HeadOutput bool
	BodyOutput bool
}

type Request struct {
	IP   string
	Conf *Config
}

type Result struct {
	IP      string
	Status  string
	ReqTime time.Duration
	Err     error
}

func (rq *Request) Call() *Result {
	res := &Result{}
	requrl := rq.Conf.URL
	urlValue, err := url.Parse(requrl)
	if err != nil {
		res.Err = err
		return res
	}
	host := urlValue.Host
	ip := rq.IP
	if ip == "" {
		ip = host
	}
	res.IP = ip
	newrequrl := strings.Replace(requrl, host, ip, 1)
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{ServerName: host},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
		Timeout:       5 * time.Second,
	}
	req, err := http.NewRequest("GET", newrequrl, nil)
	if err != nil {
		res.Err = errors.New(strings.ReplaceAll(err.Error(), newrequrl, requrl))
		return res
	}
	req.Host = host
	useragent := rq.Conf.UserAgent
	if useragent == "" {
		useragent = "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/78.0.3904.108 Safari/537.36"
	}
	req.Header.Set("USER-AGENT", useragent)
	cookiestr := rq.Conf.Cookie
	if cookiestr != "" {
		req.Header.Set("Cookie", cookiestr)
	}

	now := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		res.Err = errors.New(strings.ReplaceAll(err.Error(), newrequrl, requrl))
		return res
	}
	defer resp.Body.Close()
	res.ReqTime = time.Since(now)
	res.Status = resp.Status
	if rq.Conf.HeadOutput {
		ip = strings.ReplaceAll(ip, ":", ".")
		f, err := os.OpenFile("result/"+ip+"_header.txt", os.O_CREATE|os.O_RDWR, 0777)
		if err != nil {
			res.Err = err
			return res
		}
		defer f.Close()
		fmt.Fprintln(f, resp.Status)
		resp.Header.Write(f)
	}
	if rq.Conf.BodyOutput {
		f, err := os.OpenFile("result/"+ip+"_body.txt", os.O_CREATE|os.O_RDWR, 0777)
		if err != nil {
			res.Err = err
			return res
		}
		defer f.Close()
		io.Copy(f, resp.Body)
	}
	return res
}

func (result *Result) Print() {
	if result.Err != nil {
		fmt.Println("Request IP:" + result.IP)
		fmt.Println("Request Error:" + result.Err.Error())
		fmt.Println()
		fmt.Println("----------------------------------")

	} else {
		fmt.Println("Request IP:" + result.IP)
		fmt.Println("Response Satus:" + result.Status)
		fmt.Printf("Response Time: %.2f秒\n", result.ReqTime.Seconds())
		fmt.Println()
		fmt.Println("----------------------------------")
	}
}

func main() {
	err := EmptyDirs()
	if err != nil {
		fmt.Printf("清空result目录报错:%v\n", err)
		return
	}
	reqs, err := ParseRequestConf()
	if err != nil {
		fmt.Printf("解析request.txt报错:%v\n", err)
		return
	}
	conf := reqs[0].Conf
	out := make(chan *Result, len(reqs))
	fmt.Println("正在发送请求，请稍候...")
	fmt.Println("Request URL: " + conf.URL)
	fmt.Println("----------------------------------")
	for _, req := range reqs {
		req := req
		go func() {
			res := req.Call()
			out <- res
		}()
	}
	for i := 0; i < len(reqs); i++ {
		res := <-out
		res.Print()
	}
	if conf.HeadOutput || conf.BodyOutput {
		fmt.Println("请求结束，更详细结果，请查看result目录")
	}
	fmt.Println("请按Enter键关闭程序")
	fmt.Scanln()
}

func convToBool(val string) bool {
	val = strings.TrimSpace(val)
	if val == "" || val == "0" || strings.ToLower(val) == "false" {
		return false
	}
	return true
}

func ParseRequestConf() (reqs []*Request, err error) {
	reqs = []*Request{}
	conf := &Config{}
	f, err := os.Open("request.txt")
	if err != nil {
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		text := scanner.Text()
		if strings.HasPrefix(text, "url:") {
			conf.URL = strings.TrimSpace(text[4:])
		} else if strings.HasPrefix(text, "user-agent:") {
			conf.UserAgent = strings.TrimSpace(text[11:])
		} else if strings.HasPrefix(text, "header-output:") {
			conf.HeadOutput = convToBool(text[14:])
		} else if strings.HasPrefix(text, "body-output:") {
			conf.BodyOutput = convToBool(text[12:])
		} else if strings.HasPrefix(text, "cookie:") {
			conf.Cookie = strings.TrimSpace(text[7:])
		} else {
			req := &Request{Conf: conf}
			ip := strings.TrimSpace(text)
			req.IP = ip
			reqs = append(reqs, req)
		}
	}
	if len(reqs) == 0 {
		reqs = append(reqs, &Request{Conf: conf})
	}
	err = scanner.Err()
	return
}

func EmptyDirs() error {
	dirname := "./result"
	fileinfo, err := os.Stat(dirname)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(dirname, 0777)
		}
		return err
	}
	if !fileinfo.IsDir() {
		return errors.New("Cannot Create Dir result")
	}
	f, err := os.Open(dirname)
	if err != nil {
		return err
	}
	defer f.Close()
	names, err := f.Readdirnames(-1)
	for _, name := range names {
		os.RemoveAll(dirname + "/" + name)
	}
	return nil
}

func RequestIP(requrl string, ip string) error {
	urlValue, err := url.Parse(requrl)
	if err != nil {
		return err
	}
	host := urlValue.Host
	if ip == "" {
		ip = host
	}
	newrequrl := strings.Replace(requrl, host, ip, 1)
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{ServerName: host},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
		Timeout:       5 * time.Second,
	}
	req, err := http.NewRequest("GET", newrequrl, nil)
	if err != nil {
		return errors.New(strings.ReplaceAll(err.Error(), newrequrl, requrl))
	}
	req.Host = host
	req.Header.Set("USER-AGENT", "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/78.0.3904.108 Safari/537.36")
	now := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return errors.New(strings.ReplaceAll(err.Error(), newrequrl, requrl))
	}
	defer resp.Body.Close()
	fmt.Println("Request IP:" + ip)
	fmt.Println("Response Satus:" + resp.Status)
	fmt.Printf("Response Time: %.2f秒\n", time.Since(now).Seconds())
	fmt.Println()
	fmt.Println("----------------------------------")
	ip = strings.ReplaceAll(ip, ":", ".")
	f, err := os.OpenFile("result/"+ip+"_header.txt", os.O_CREATE|os.O_RDWR, 0777)
	if err != nil {
		return err
	}
	defer f.Close()
	fmt.Fprintln(f, resp.Status)
	resp.Header.Write(f)
	f, err = os.OpenFile("result/"+ip+"_body.txt", os.O_CREATE|os.O_RDWR, 0777)
	if err != nil {
		return err
	}
	defer f.Close()
	io.Copy(f, resp.Body)
	return nil
}
