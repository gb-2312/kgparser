// 抓取女神-全民K歌|唱吧主页的所有歌曲
package music

import (
	"container/list"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

// 定义常量
const (
	NORMAL_SLASHES                          = "/"
	MINUS_SIGN                              = "-"
	DEFAULT_CHARSET                         = "utf-8"
	ZERO                                    = 0
	ERROR                                   = -1
	NOT_FOUND                               = ERROR
	EMPTY_STR                               = ""
	NEXT_LINE_CHAR                          = '\n'
	QUESTION_STR                            = "?"
	DOT_STR                                 = "."
	HTTP_PREFIX                             = "http://"
	KG_HOST                                 = "node.kg.qq.com"
	KG_HOME_PAGE_URL                        = HTTP_PREFIX + KG_HOST + "/cgi/fcgi-bin/kg_ugc_get_homepage?"
	KG_PLAY_URL_TEMPLATE                    = HTTP_PREFIX + KG_HOST + "/play?s=%s&g_f=personal"
	KG_JSON_CALLBACK_RESULT_PREFIX          = "MusicJsonCallback("
	KG_JSON_CALLBACK_RESULT_SUFFIX          = ")"
	HAS_MORE_FINISH_SIGN                    = 0
	DEFAULT_USER_AGENT                      = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_13_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/71.0.3578.80 Safari/537.36"
	DEFAULT_ACCEPT_STRING                   = "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8"
	DEFAULT_ACCEPT_LANGUAGE                 = "zh-CN,zh;q=0.9,en;q=0.8"
	DEFAULT_CONNECTION_TYPE                 = "keep-alive"
	KG_DEFAULT_MEDIA_SUFFIX                 = ".m4a"
	UNKNOW_SINGER_NAME                      = "佚名"
	CB_HOST                                 = "changba.com"
	CB_USER_ID_URL_TEMPLATE                 = HTTP_PREFIX + CB_HOST + "/u/%s"
	CB_PLAY_URL_TEMPLATE                    = HTTP_PREFIX + CB_HOST + "/s/%s"
	DOWNLOAD_MEDIA_TIPS_TEMPLATE            = "准备下载资源: %s, 演唱者: %s, 资源地址: %s"
	TRACE_METHOD_COST_TIME_TEMPLATE         = "execute %s cost time: (%s)%s"
	FINISH_DOWNLOAD_MEDIA_FILES_TEMPLATE    = "歌曲全部下载完毕!"
	PARSE_NOT_ALLOWED_REPEAT_TIPES_TEMPLATE = "parser已经正在运行中, 请不要重复调用! %s"
	INPUT_DEFAULT_TIPS_TEMPLATE             = "请输入全民K歌|唱吧的地址: "
	REPEAT_INPUT_DEFAULT_TIPS_TEMPLATE      = "请确认输入全民K歌|唱吧的地址: "
	CB_MATCH_INPUT_STR                      = CB_HOST + "/u/"
	KG_MATCH_INPUT_STR                      = KG_HOST + "/personal?uid="
	CB_DEFAULT_AUDIO_STR                    = ".mp3"
	CB_DEFAULT_VIDEO_SUFFIX                 = ".mp4"
	CB_VIDEO_RESOURCE_URL_TEMPLATE          = "http://letv.cdn.changba.com/userdata/video/%s%s"
	KG_MIN_UID_LEN                          = 10
	CB_MIN_UID_LEN                          = 8
	FILE_ALREADY_EXISTS_TIPS_TEMPLATE       = "文件已经存在: %s"
	CB_LOAD_MORE_URL_TEMPLATE               = HTTP_PREFIX + CB_HOST + "/member/personcenter/loadmore.php?ver=1&type=0&curuserid=-1&pageNum=%s&userid=%s"
	CB_DOWNLOAD_MEDIA_FAIL_TEMPLATE         = "资源下载失败! 资源名称: %s"
	WINDOWS_OS_SLASHES                      = "\\"
)

// Method-Type
type MethodType string

// 定义常用的HTTP_METHOD类型
const (
	//GET
	GET MethodType = "GET"
	// POST
	POST MethodType = "POST"
)

// define: ParserStatus
type ParserStatus int

// 解析器自身的状态
const (
	// 默认
	NONE ParserStatus = iota
	// 运行中
	RUNNING
	// 下载资源中
	DOWNLOADING
	// 空闲
	FREE
	// 终止
	TERMINATED
)

// 定义全局变量
var (
	HEARDER_PARAMS = map[string]string{
		"Accept":          DEFAULT_ACCEPT_STRING,
		"Accept-Language": DEFAULT_ACCEPT_LANGUAGE,
		"Host":            KG_HOST,
		"Upgrade-Insecure-Requests": "1",
		"Connection":                DEFAULT_CONNECTION_TYPE,
		"User-Agent":                DEFAULT_USER_AGENT,
	}
)

// read-write object
type RWDict struct {
	// 针对map的读写锁
	rwLock sync.RWMutex
	// mapping-data
	mapping map[string]interface{}
}

// put
// @param k key
// @param v value
func (rw *RWDict) Put(k string, v interface{}) {
	rw.rwLock.Lock()
	defer rw.rwLock.Unlock()
	rw.mapping[k] = v
}

// get
// @param k key
// @return value
func (rw *RWDict) Get(k string) interface{} {
	rw.rwLock.RLock()
	defer rw.rwLock.RUnlock()
	v, _ := rw.mapping[k]
	return v
}

// remove
// @param k key
func (rw *RWDict) Remove(k string) {
	rw.rwLock.Lock()
	defer rw.rwLock.Unlock()
	delete(rw.mapping, k)
}

// 解析器的接口
type Parser interface {
	// 设定ParserStatus
	// @param ParserStatus
	SetParserStatus(status ParserStatus)

	// 初始化
	Initial()

	// 销毁
	Destroy()

	// 获取当前parser的状态
	// @return ParserStatus
	GetParserStatus() ParserStatus

	// 根据uid, 获得所有的歌曲信息+下载
	// @param uid 用户的id
	GetMusic(uid string)

	// 获取播放列表
	// @param uid 用户的id
	// @return 播放列表List<`UgcItem`>对象
	GetPlayerList(uid string) (*list.List)

	// (protected方法)post请求
	// post(Call:: HttpClient.func())
	// @param method_type MethodType
	// @param request_url POST请求地址
	// @param params 请求参数字典
	// @param cookies 所有的cookie
	// @return <body内容, error>
	post(method_type MethodType, request_url string, params map[string]string, cookies []*http.Cookie) ([]byte, error)

	// (protected方法)统计方法执行时间
	// @param func_name 方法名称
	// @return func
	trace(func_name string) func()

	// 根据share_id, 构造播放该Item的地址
	// @param share_id ShareId
	// @param item item
	// @return 播放UgcItem基础信息、参数的地址
	GetPlayUrl(share_id string, item interface{}) string

	// 下载媒体
	// @param url 播放UgcItem基础信息、参数的地址
	// @param item item
	DownloadMedia(url string, item interface{})

	// 根据播放UgcItem基础信息、参数的地址, 获取实际所需的下载音乐文件的URL、昵称、专辑名称
	// @param url 播放UgcItem基础信息、参数的地址
	// @return 正则结果数组: 占位符结果分组group
	GetItemArguments(url string) []string

	// 根据歌曲文件下载链接, 动态获取实际上传到全民K歌CDN服务器的媒体后缀类型
	// @param song_url 歌曲的地址
	// @return 歌曲的实际后缀
	GetDownloadMediaSuffix(song_url string) string
}

// 抽象的Parser定义
type AbstractParser struct {
	// 组合接口
	Parser
	// 当前Parser的状态
	status ParserStatus
	// HttpClient-adapter
	client HttpClient
	// cookies
	cookies []*http.Cookie
}

func (parser *AbstractParser) SetParserStatus(status ParserStatus) {
	parser.status = status
}

func (parser *AbstractParser) Initial() {
	if parser.GetParserStatus() != NONE {
		return
	}
	parser.client = postSimpleReq
	parser.status = RUNNING
}

func (parser *AbstractParser) Destroy() {
	if parser.GetParserStatus() == TERMINATED {
		return
	}
	parser.status = TERMINATED
	os.Exit(ZERO)

}

func (parser *AbstractParser) GetParserStatus() ParserStatus {
	return parser.status
}

// check-status
// @param func-name
// @return True: allow; False: not-allowed
func (parser *AbstractParser) checkStatus(func_name string) bool {
	if parser.GetParserStatus() != RUNNING && parser.GetParserStatus() != FREE {
		fmt.Println(fmt.Sprintf(PARSE_NOT_ALLOWED_REPEAT_TIPES_TEMPLATE, func_name))
		return false
	}
	return true
}

func (parser *AbstractParser) post(method_type MethodType, request_url string, params map[string]string, cookies []*http.Cookie) ([]byte, error) {
	body, cookies, err := post(parser.client, method_type, request_url, params, cookies)
	if err == nil && cookies != nil {
		parser.cookies = cookies
	}
	return body, err
}

func (parser *AbstractParser) trace(func_name string) func() {
	start := time.Now()
	return func() {
		fmt.Printf(TRACE_METHOD_COST_TIME_TEMPLATE, func_name, time.Since(start), string(NEXT_LINE_CHAR))
	}
}

// 根据歌手名称, 创建歌手目录
// @param singer 歌手名称
// @return 实际的歌曲存放目录
func (parser *AbstractParser) GetDownloadPath(singer string) string {
	return dotStrWithOsSlashes() + singer
}

// 拼写SongPath
// @param download_path 下载位置
// @param song_name 歌曲名称
// @param singer 歌手名称
// @param media_type 下载的媒体类型
// @return 拼写完成的SongPath
func (parser *AbstractParser) GetSongPath(download_path string, song_name string, singer string, media_type string) string {
	return download_path + getOsSlashes() + song_name + MINUS_SIGN + singer + media_type
}

// 检查 AND 写入为媒体文件
// @param download_path 下载位置
// @param song_name 歌曲名称
// @param song_url 歌曲地址
func (parser *AbstractParser) checkAndWriteMedia(download_path string, song_path string, song_url string) {
	parser.checkFolder(download_path)
	parser.checkMedia(song_path, song_url)
}

// 检测/创建目标文件夹/目录
// @param download_path 下载位置
func (parser *AbstractParser) checkFolder(download_path string) {
	// 检查路径, 如果不存在, 则创建
	path_exist, err := pathExists(download_path)
	if err != nil {
		log.Fatal(err)
		parser.status = TERMINATED
	}
	if !path_exist {
		err := os.Mkdir(download_path, os.ModePerm)
		if err != nil {
			log.Fatal(err)
			parser.status = TERMINATED
			// 创建失败, 则关闭Golang Runtime!
			os.Exit(ERROR)
		}
	}
}

// 检测/创建目标媒体文件
// @param song_path 歌曲的完整位置
// @param song_url 歌曲地址
func (parser *AbstractParser) checkMedia(song_path string, song_url string) {
	// 是否要下载该文件
	wasDownloadFile := true

	// 检查即将写入的下载文件, 如果存在, 则忽略; 否则下载并写入该空的文件中
	file_exists, err := pathExists(song_path)
	if err != nil {
		log.Fatal(err)
		parser.status = TERMINATED
		wasDownloadFile = false
	}
	if file_exists {
		fmt.Println(fmt.Sprintf(FILE_ALREADY_EXISTS_TIPS_TEMPLATE, song_path))
		wasDownloadFile = false
	}

	if !wasDownloadFile {
		return
	}

	// download_file(prepared to write byte[] data)
	out, err := os.Create(song_path)
	if err != nil {
		log.Fatal(err)
		parser.status = TERMINATED
	}
	defer out.Close()

	// 此处使用普通的Get下载
	resp, err := http.Get(song_url)
	if err != nil {
		log.Fatal(err)
		parser.status = TERMINATED
	}
	defer resp.Body.Close()

	io.Copy(out, resp.Body)
}

// 兼容postSimpleReq函数
// @param method_type MethodType
// @param request_url POST请求地址
// @param params 请求参数字典
// @param cookies 所有的cookies
// @return <body内容, []cookie, error>
type HttpClient func(method_type MethodType, request_url string, params map[string]string, cookies []*http.Cookie) ([]byte, []*http.Cookie, error)

// post(Call:: HttpClient.func())
// @param HttpClient(func)
// @param method_type MethodType
// @param request_url POST请求地址
// @param params 请求参数字典
// @param cookies 所有的cookies
// @return <body内容, []cookie, error>
func post(client HttpClient, method_type MethodType, request_url string, params map[string]string, cookies []*http.Cookie) ([]byte, []*http.Cookie, error) {
	return client(method_type, request_url, params, cookies)
}

// 封装为简单的POST请求
// @param method_type MethodType
// @param request_url POST请求地址
// @param params 请求参数字典
// @param cookies 所有的cookies
// @return <body内容, []cookie, error>
func postSimpleReq(method_type MethodType, request_url string, params map[string]string, cookies []*http.Cookie) ([]byte, []*http.Cookie, error) {
	http_client := &http.Client{}
	request_params := url.Values{}

	if params != nil {
		for k, v := range params {
			request_params.Add(k, v)
		}
	}

	req, err := http.NewRequest(string(method_type), request_url, strings.NewReader(request_params.Encode()))
	for k, v := range HEARDER_PARAMS {
		req.Header.Set(k, v)
	}

	if cookies != nil {
		for _, v := range cookies {
			req.AddCookie(v)
		}
	}

	resp, err := http_client.Do(req)
	defer resp.Body.Close()
	if err != nil {
		return nil, nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	return body, resp.Cookies(), nil
}

// 判定是否存在文件、文件夹
// @param path 被判定的: 文件/文件夹
// @return <是否存在, error>
func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// get caller-funcName
// @return function-name
func funcName() string {
	pc, _, _, _ := runtime.Caller(1)
	return runtime.FuncForPC(pc).Name()
}

// 只针对windows操作系统特殊处理
// @return osSlashes
func getOsSlashes() string {
	if runtime.GOOS == "windows" {
		return WINDOWS_OS_SLASHES
	}
	return NORMAL_SLASHES
}

// DOT_STR_WITH_OS_SLASHES
// @return DOT_STR_WITH_OS_SLASHES
func dotStrWithOsSlashes() string {
	return DOT_STR + getOsSlashes()
}
