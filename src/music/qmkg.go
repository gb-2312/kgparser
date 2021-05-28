// qmkg的解析实现
package music

import (
	"container/list"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// 定义常量
const (
	GET_ITEM_ARGUMENT_MATCH_STR = "window.__DATA__.*?kg_nick\":\"(.*?)\",\".*?playurl\":\"(.*?)\",\".*?song_name\":\"(.*?)\""
	QMKG_1ST_MEDIA_MATCH_STR    = "fname=(.*?)&fromtag"
	QMKG_RETRY_MEDIA_MATCH_STR  = "tc.qq.com/(.*?)vkey"
)

// 全民K歌的解析器
type UgcParser struct {
	// 组合解析器的接口
	AbstractParser
	// HttpClient-adapter
	client HttpClient
}

// 全民K歌的歌曲单元
type UgcItem struct {
	// 头像地址
	Avatar string `json:avatar`
	// shareId
	ShareId string `json:"shareid"`
	// 歌曲发布时间
	Time int64 `json:"time"`
	// 歌曲标题
	Title string `json:"title"`
}

// 全民K歌的歌曲整体的Data数据
type UgcData struct {
	// 是否还有可加载的歌曲
	HasMore int `json:"has_more"`
	// 歌曲列表
	UgcList []UgcItem `json:"ugclist"`
}

// 全民K歌的整体的UgcBody
type UgcBody struct {
	// 歌曲整体的Data数据
	Data UgcData
}

func (parser *UgcParser) SetParserStatus(status ParserStatus) {
	parser.AbstractParser.SetParserStatus(status)
}

func (parser *UgcParser) Initial() {
	parser.AbstractParser.Initial()
}

func (parser *UgcParser) Destroy() {
	parser.AbstractParser.Destroy()
}

func (parser *UgcParser) GetParserStatus() ParserStatus {
	return parser.status
}

func (parser *UgcParser) GetMusic(uid string) {
	func_name := funcName()
	if !parser.checkStatus(func_name) {
		return
	}
	parser.status = DOWNLOADING
	defer func() {
		parser.status = FREE
	}()
	ugc_item_list := parser.GetPlayerList(uid)

	var wg sync.WaitGroup
	wg.Add(ugc_item_list.Len())
	defer parser.trace(func_name)()

	for e := ugc_item_list.Front(); e != nil; e = e.Next() {
		go func(item UgcItem) {
			defer wg.Done()
			share_id := item.ShareId
			url := parser.GetPlayUrl(share_id, item)
			parser.DownloadMedia(url, item)
		}(e.Value.(UgcItem))
	}
	wg.Wait()
	fmt.Println(FINISH_DOWNLOAD_MEDIA_FILES_TEMPLATE)
}

func (parser *UgcParser) GetPlayerList(uid string) *list.List {
	time := 1
	ugc_item_list := list.New()
	for {
		params := map[string]string{
			"outCharset": DEFAULT_CHARSET,
			"format":     "jsonp",
			"type":       "get_ugc",
			"start":      strconv.Itoa(time),
			"num":        "8",
			"share_uid":  uid}

		body, err := parser.post(POST, KG_HOME_PAGE_URL, params, nil)
		if err != nil {
			log.Fatal(err)
			parser.status = TERMINATED
		}

		info := body[len(KG_JSON_CALLBACK_RESULT_PREFIX) : len(body)-len(KG_JSON_CALLBACK_RESULT_SUFFIX)]
		var ugc_body *UgcBody
		if err = json.Unmarshal([]byte(info), &ugc_body); err != nil {
			log.Fatal(err)
			parser.status = TERMINATED
			break
		}

		ugc_data := ugc_body.Data

		time++
		for _, ugc_item := range ugc_data.UgcList {
			ugc_item_list.PushBack(ugc_item)
		}
		if ugc_data.HasMore == HAS_MORE_FINISH_SIGN {
			break
		}
	}
	return ugc_item_list
}

func (parser *UgcParser) post(method_type MethodType, request_url string, params map[string]string, cookies []*http.Cookie) ([]byte, error) {
	return parser.AbstractParser.post(method_type, request_url, params, cookies)
}

func (parser *UgcParser) trace(func_name string) func() {
	return parser.AbstractParser.trace(func_name)
}

func (parser *UgcParser) GetPlayUrl(share_id string, item interface{}) string {
	return fmt.Sprintf(KG_PLAY_URL_TEMPLATE, share_id)
}

func (parser *UgcParser) DownloadMedia(url string, item interface{}) {
	// 正则结果
	find_result := parser.GetItemArguments(url)
	// 歌手名
	singer := find_result[1]
	// 歌曲实际下载地址
	song_url := find_result[2]
	// 歌曲名
	song_name := find_result[3]
	// print informations
	fmt.Println(fmt.Sprintf(DOWNLOAD_MEDIA_TIPS_TEMPLATE, song_name, singer, song_url))
	// 媒体类型
	media_type := parser.GetDownloadMediaSuffix(song_url)
	// 下载路径
	download_path := parser.AbstractParser.GetDownloadPath(singer)
	// 歌曲保存路径
	song_path := parser.AbstractParser.GetSongPath(download_path, song_name, singer, media_type)
	// 检测文件夹/媒体文件(已存在则不会有任何的操作)
	parser.AbstractParser.checkAndWriteMedia(download_path, song_path, song_url)
}

func (parser *UgcParser) GetItemArguments(url string) []string {
	body, err := parser.post(POST, url, nil, nil)
	if err != nil {
		log.Fatal(err)
		parser.status = TERMINATED
	}

	reg := regexp.MustCompile(GET_ITEM_ARGUMENT_MATCH_STR)
	return reg.FindStringSubmatch(string(reg.Find(body)))
}

func (parser *UgcParser) GetDownloadMediaSuffix(song_url string) string {
	reg := regexp.MustCompile(QMKG_1ST_MEDIA_MATCH_STR)
	media_type_group := reg.FindStringSubmatch(string(reg.Find([]byte(song_url))))
	media_type := EMPTY_STR
	if media_type_group == nil {
		reg = regexp.MustCompile(QMKG_RETRY_MEDIA_MATCH_STR)
		media_type_group = reg.FindStringSubmatch(string(reg.Find([]byte(song_url))))
	}
	if media_type_group != nil {
		media_type = media_type_group[1]
		media_type = media_type[strings.LastIndex(media_type, DOT_STR):]
		if media_type != EMPTY_STR {
			question_pos := strings.LastIndex(media_type, QUESTION_STR)
			if question_pos != NOT_FOUND {
				media_type = media_type[:question_pos]
			}
		}
	}
	if media_type == EMPTY_STR {
		media_type = KG_DEFAULT_MEDIA_SUFFIX
	}
	return media_type
}
