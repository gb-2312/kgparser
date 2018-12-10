// changba的解析实现
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
	CB_1ST_MEDIA_MATCH_STR           = HTTP_PREFIX + "(\\S+)." + CB_HOST + "/(\\S+)(\\d+)" + CB_DEFAULT_AUDIO_STR
	CB_RETRY_MEDIA_MATCH_STR         = HTTP_PREFIX + "http://(\\S+)." + CB_HOST + "/userdata/userwork/(\\S+)(\\.*)" + CB_DEFAULT_AUDIO_STR
	CB_SINGER_NAME_MATCH_STR         = "class=\"uname twemoji\" style=\"display: inline!important;\"(.*?)</a>"
	CB_SINGER_NAME_CONTENT_START_STR = "target=\"_blank\">"
	CB_SINGER_NAME_CONTENT_END_STR   = "</a>"
	CB_USER_ID_MATCH_STR             = "var userid = '(\\d+)'"
)

// 定义全局变量
var (
	DEFAULT_COOKIES = []*http.Cookie{
		&http.Cookie{Name: "appver", Value: "1.2.1", HttpOnly: true},
		&http.Cookie{Name: "os", Value: "osx", HttpOnly: true},
	}
)

// is-mv?
type IsMv string

// 定义mv类型
const (
	// 不是mv
	MV_NONE IsMv = "none"
	// 是mv
	MV_INLINE IsMv = "inline"
)

// Changba的解析器
type CbParser struct {
	// 组合解析器的接口
	AbstractParser
	//// 歌曲mapping关联
	//songMapping map[string]CbItem
	//// 针对map的读写锁
	//rwLock sync.RWMutex
	rwDict RWDict
}

// Changba的歌曲单元
type CbItem struct {
	// ChangBa歌曲名称
	SongName string `json:songname`
	// 表示是否mv
	IsMv string `json:"ismv"`
	// 获取mv地址或者歌曲mp3文件
	WorkId string `json:"workid"`
	// 获取作品首页
	EnWorkId string `json:"enworkid"`
}

func (parser *CbParser) SetParserStatus(status ParserStatus) {
	parser.AbstractParser.SetParserStatus(status)
}

func (parser *CbParser) Initial() {
	parser.AbstractParser.Initial()
	parser.rwDict.mapping = make(map[string]interface{})
}

func (parser *CbParser) Destroy() {
	parser.AbstractParser.Destroy()
}

func (parser *CbParser) GetParserStatus() ParserStatus {
	return parser.status
}

func (parser *CbParser) GetMusic(uid string) {
	uid = parser.getUserId(uid)
	func_name := funcName()
	if !parser.checkStatus(func_name) {
		return
	}
	parser.status = DOWNLOADING
	defer func() {
		parser.status = FREE
	}()
	cb_item_list := parser.GetPlayerList(uid)

	var wg sync.WaitGroup
	wg.Add(cb_item_list.Len())
	defer parser.trace(func_name)()

	for e := cb_item_list.Front(); e != nil; e = e.Next() {
		go func(item CbItem) {
			defer wg.Done()
			share_id := item.EnWorkId
			url := parser.GetPlayUrl(share_id, item)
			parser.DownloadMedia(url, item)
		}(e.Value.(CbItem))
	}
	wg.Wait()
	fmt.Println(FINISH_DOWNLOAD_MEDIA_FILES_TEMPLATE)
}

func (parser *CbParser) GetPlayerList(uid string) (*list.List) {
	cb_item_list := list.New()

	page_number := 0
	for {
		body, err := parser.post(GET, fmt.Sprintf(CB_LOAD_MORE_URL_TEMPLATE, strconv.Itoa(page_number), uid), nil, DEFAULT_COOKIES)
		if err != nil {
			log.Fatal(err)
			return nil
		}

		var cbItemArr []CbItem
		if err = json.Unmarshal([]byte(body), &cbItemArr); err != nil {
			log.Fatal(err)
			break
		}

		if cbItemArr != nil && len(cbItemArr) > 0 {
			for _, v := range cbItemArr {
				cb_item_list.PushBack(v)
			}
		} else {
			break
		}

		page_number++
	}
	return cb_item_list
}

func (parser *CbParser) post(method_type MethodType, request_url string, params map[string]string, cookies []*http.Cookie) ([]byte, error) {
	return parser.AbstractParser.post(method_type, request_url, params, cookies)
}

func (parser *CbParser) trace(func_name string) func() {
	return parser.AbstractParser.trace(func_name)
}

func (parser *CbParser) GetPlayUrl(share_id string, item interface{}) string {
	url := fmt.Sprintf(CB_PLAY_URL_TEMPLATE, share_id)
	switch it := item.(type) {
	case CbItem:
		// FIXME 唱吧MV暂时不可用
		isMv := strings.Index(it.IsMv, string(MV_INLINE)) != NOT_FOUND
		if isMv {
			url = fmt.Sprintf(CB_VIDEO_RESOURCE_URL_TEMPLATE, it.EnWorkId, CB_DEFAULT_VIDEO_SUFFIX)
		}
		break
	}
	parser.rwDict.Put(url, item)
	return url
}

func (parser *CbParser) DownloadMedia(url string, item interface{}) {
	switch it := item.(type) {
	case CbItem:
		isMv := strings.Index(it.IsMv, string(MV_INLINE)) != NOT_FOUND
		if isMv {
			parser.downloadMp4(url)
			return
		}
		break
	}
	body, err := parser.post(GET, url, nil, nil)
	if err != nil {
		log.Fatal(err)
	}
	reg := regexp.MustCompile(CB_1ST_MEDIA_MATCH_STR)
	content_match_group := reg.FindStringSubmatch(string(reg.Find(body)))
	if content_match_group == nil {
		reg := regexp.MustCompile(CB_RETRY_MEDIA_MATCH_STR)
		content_match_group = reg.FindStringSubmatch(string(reg.Find(body)))
	}

	cb_item := parser.rwDict.Get(url).(CbItem)
	song_name := cb_item.SongName
	if content_match_group == nil {
		fmt.Println(fmt.Sprintf(CB_DOWNLOAD_MEDIA_FAIL_TEMPLATE, song_name))
		return
	}
	song_url := content_match_group[0]

	singer := EMPTY_STR
	reg = regexp.MustCompile(CB_SINGER_NAME_MATCH_STR)
	singer_match_group := reg.FindStringSubmatch(string(reg.Find(body)))
	if singer_match_group == nil {
		singer = UNKNOW_SINGER_NAME
	} else {
		singer = singer_match_group[0]
		target_start_sign := CB_SINGER_NAME_CONTENT_START_STR
		target_end_sign := CB_SINGER_NAME_CONTENT_END_STR
		singer = singer[strings.LastIndex(singer, target_start_sign)+len(target_start_sign):]
		singer = singer[:len(singer)-len(target_end_sign)]
	}

	media_type := parser.GetDownloadMediaSuffix(song_url)
	fmt.Println(fmt.Sprintf(DOWNLOAD_MEDIA_TIPS_TEMPLATE, song_name, singer, song_url))

	// 下载路径
	download_path := parser.AbstractParser.GetDownloadPath(singer)
	// 歌曲保存路径
	song_path := parser.AbstractParser.GetSongPath(download_path, song_name, singer, media_type)
	// 检测文件夹/媒体文件(已存在则不会有任何的操作)
	parser.AbstractParser.checkAndWriteMedia(download_path, song_path, song_url)
}

// Deprecated
// (protected方法)下载mp4文件
// @param song_url 歌曲下载地址
func (parser *CbParser) downloadMp4(song_url string) {
	cb_item := parser.rwDict.Get(song_url).(CbItem)

	singer := UNKNOW_SINGER_NAME
	song_name := cb_item.SongName
	media_type := parser.GetDownloadMediaSuffix(song_url)
	fmt.Println(fmt.Sprintf(DOWNLOAD_MEDIA_TIPS_TEMPLATE, song_name, singer, song_url))

	// 下载路径
	download_path := parser.AbstractParser.GetDownloadPath(singer)
	// 歌曲保存路径
	song_path := parser.AbstractParser.GetSongPath(download_path, song_name, singer, media_type)
	// 检测文件夹/媒体文件(已存在则不会有任何的操作)
	parser.AbstractParser.checkAndWriteMedia(download_path, song_path, song_url)
}

// (protected方法)根据uid, 获取changba实际的userId
// @param uid 原始的uid
// @return 获得实际对应的UserId
func (parser *CbParser) getUserId(uid string) string {
	request_url := fmt.Sprintf(CB_USER_ID_URL_TEMPLATE, uid)
	body, err := parser.post(GET, request_url, HEARDER_PARAMS, nil)
	if err != nil {
		log.Fatal(err)
	}
	reg := regexp.MustCompile(CB_USER_ID_MATCH_STR)
	results := reg.FindStringSubmatch(string(body))
	if results != nil {
		return results[1]
	}
	return EMPTY_STR
}

func (parser *CbParser) GetDownloadMediaSuffix(song_url string) string {
	return song_url[strings.LastIndex(song_url, DOT_STR):]
}
