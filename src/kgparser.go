package main

import (
	"bufio"
	"fmt"
	"kgparser/src/music"
	"os"
	"regexp"
	"strings"
	"sync"
)

// 定义常量
const (
	CONSOLE_INPUT_MATCH_STR = "\\w{%d,}"
)

// 定义全局变量
var (
	// sync.once
	once sync.Once
	// singleton selfUgcParser
	selfUgcParser *music.UgcParser
	// singleton selfCbParser
	selfCbParser *music.CbParser
)

// 获取UgcParser的单例对象
// @return UgcParser单例对象
func GetUgcInstance() *music.UgcParser {
	return selfUgcParser
}

// 获取CbParser的单例对象
// @return CbParser单例对象
func GetCbInstance() *music.CbParser {
	return selfCbParser
}

// init-method
func init() {
	once.Do(func() {
		selfUgcParser = &music.UgcParser{}
		selfUgcParser.Initial()

		selfCbParser = &music.CbParser{}
		selfCbParser.Initial()
	})
}

// main-method!
func main() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(music.INPUT_DEFAULT_TIPS_TEMPLATE)
	var match_group []string
	var was_changba = false
	var was_qmkg = false

	for {
		input_text, _ := reader.ReadString(music.NEXT_LINE_CHAR)
		if input_text == music.EMPTY_STR {
			fmt.Print(music.REPEAT_INPUT_DEFAULT_TIPS_TEMPLATE)
			continue
		}
		was_changba = strings.Index(input_text, music.CB_MATCH_INPUT_STR) != music.NOT_FOUND
		was_qmkg = strings.Index(input_text, music.KG_MATCH_INPUT_STR) != music.NOT_FOUND
		if !was_changba && !was_qmkg {
			fmt.Print(music.REPEAT_INPUT_DEFAULT_TIPS_TEMPLATE)
			continue
		}

		reg_len := music.CB_MIN_UID_LEN
		if was_qmkg {
			reg_len = music.KG_MIN_UID_LEN
		}
		reg := regexp.MustCompile(fmt.Sprintf(CONSOLE_INPUT_MATCH_STR, reg_len))
		match_group = reg.FindStringSubmatch(input_text)

		if match_group != nil {
			break
		} else {
			fmt.Print(music.REPEAT_INPUT_DEFAULT_TIPS_TEMPLATE)
			continue
		}
	}

	uid := match_group[0]
	if was_qmkg {
		GetUgcInstance().GetMusic(uid)
	} else if was_changba {
		GetCbInstance().GetMusic(uid)
	}
}
