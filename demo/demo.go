package main

import (
	"context"
	"fmt"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/hsw409328/chromedp"
	"github.com/hsw409328/gofunc"
	"log"
	"path"
	"strings"
)

func filterImageOrCssOrJs(urlStr string) string {
	filterExt := map[string]int{
		".jpg": 1, ".css": 1, ".js": 1, ".gif": 1, ".png": 1, ".ico": 1,
	}
	if _, ok := filterExt[strings.ToLower(path.Ext(urlStr))]; ok {
		return ""
	}
	return urlStr
}

// 获取服务列表
func run(siteUrl string, res *[]map[string]string, evalSlice *[]string, ajaxGetUrl *[]string, ajaxPostUrl *[]string) chromedp.Tasks {
	return chromedp.Tasks{
		chromedp.ActionFunc(func(_ context.Context, h cdp.Executor) error {
			go func() {
				for evt := range h.(*chromedp.TargetHandler).NetEvents() {
					switch e := evt.(type) {
					case *network.EventRequestWillBeSent:
						if strings.ToLower(e.Request.Method) == "post" {
							*ajaxGetUrl = append(*ajaxGetUrl, filterImageOrCssOrJs(e.Request.URL))
						} else if strings.ToLower(e.Request.Method) == "get" {
							*ajaxPostUrl = append(*ajaxPostUrl, filterImageOrCssOrJs(e.Request.URL))
						}
					case *network.EventResponseReceived:
						log.Printf("Got %d response from %s", e.Response.Status, e.Response.URL)
					}
				}
			}()
			return nil
		}),
		// 访问服务列表
		chromedp.Navigate(siteUrl),
		//chromedp.ByQuery 通过某个标签，获取一条记录
		//chromedp.ByID 通过标签某个ID获取一条记录
		//chromedp.ByQueryAll 通过某个标签，获取该标签所有的记录
		//chromedp.BySearch 通过搜索获取所有数据
		//以上使用方式，请参考jquery选择器

		// 等待直到body加载完毕
		chromedp.WaitReady("body", chromedp.ByQuery),
		// 等待列表渲染
		//chromedp.Sleep(2 * time.Second),
		//获取网页所有A标签
		chromedp.AttributesAll("a", res, chromedp.ByQueryAll),
		//点击网页第一个A连接
		//chromedp.Click("a", chromedp.ByQuery),
		//点击网页FORM标签
		//chromedp.AttributesAll("form", res, chromedp.ByQueryAll),

		chromedp.Evaluate(`try {
           window.alert = function(msg) {};
           window.confirm = function(msg) {
               return false
           };
           window.prompt = function(text, defaultText) {
               return false
           };
           window.close = function() {
               return false
           };
           window.history.back = function(args) {
               console.log(args)
           };
           window.history.forward = function(args) {
               console.log(args)
           };
           var eles = document.getElementsByTagName('*');
           for (x in eles) {
               elm = eles[x];
               var flag = 0;
               for (i in elm) {
                   if (i.match(/^on(\w+)$/)) {
                       if (elm[i]) {
                           try {
                               flag = 1;
                               elm[i]()
                           } catch (err) {}
                       }
                   }
               }
               if (flag == 0){
                   if (elm.tagName == 'I' || elm.tagName == 'BUTTON') {
                       try {
                           elm.click()
                       } catch (err) {}
                   }
               }
           }
       } catch (err) {
           console.log(err)
       }
		`, &evalSlice),
	}
}

func main() {
	var err error
	//siteUrl := "http://testphp.vulnweb.com"
	siteUrl := "http://security.jd.com"
	siteDomain, err := gofunc.GetDomain(siteUrl)
	level := 3
	// 全局URL
	var mainPoolRequestUrl = make(map[string]int)
	if err != nil {
		panic("Request Url Error " + err.Error())
	}

	// create context
	ctxt, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create chrome instance
	c, err := chromedp.New(ctxt /*, chromedp.WithLog(log.Printf)*/)
	if err != nil {
		log.Fatal(err)
	}

	for i := 0; i <= level; i++ {
		// 成功取得HTML内容进行后续处理
		poolRequestUrl := map[string]int{siteUrl: 1}
		for k, _ := range poolRequestUrl {
			var aMap []map[string]string
			var evalSlice []string
			var ajaxGetUrl []string
			var ajaxPostUrl []string

			// cdp是chromedp实例
			// ctx是创建cdp时使用的context.Context
			if i == 0 {
				err = c.Run(ctxt, run(siteUrl, &aMap, &evalSlice, &ajaxGetUrl, &ajaxPostUrl))
			} else {
				err = c.Run(ctxt, run(k, &aMap, &evalSlice, &ajaxGetUrl, &ajaxPostUrl))
			}
			if err != nil {
				//panic(err)
			}

			for _, v := range aMap {
				// 判断是否为URL 如果是则添加到二次被请求池 如果不是则将siteUrl连接，再进行判断
				if gofunc.IsUrl(v["href"]) {
					poolRequestUrl[v["href"]] = 1
				} else if gofunc.IsUrl(siteUrl + "/" + v["href"]) {
					poolRequestUrl[siteUrl+"/"+v["href"]] = 1
				} else {
					log.Println(v)
				}
			}
			for _, v := range ajaxGetUrl {
				poolRequestUrl[v] = 1
			}
			for _, v := range ajaxPostUrl {
				poolRequestUrl[v] = 1
			}
			// 过虑不是siteUrl的连接
			for k, _ := range poolRequestUrl {
				str, err := gofunc.GetDomain(k)
				if err != nil {
					delete(poolRequestUrl, k)
					continue
				}
				if str != siteDomain {
					delete(poolRequestUrl, k)
					continue
				}
			}
			delete(poolRequestUrl, siteUrl)
			fmt.Println("第【", i, "次运行】")
			fmt.Println(poolRequestUrl)
		}
		for k, _ := range poolRequestUrl {
			mainPoolRequestUrl[k] = 1
		}
	}

	err = c.Shutdown(ctxt)
	if err != nil {
		panic(err)
	}
	fmt.Println(mainPoolRequestUrl)
}
