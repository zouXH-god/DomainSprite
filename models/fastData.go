package models

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

type FastData struct {
	Token      string     `json:"token"`
	RecordInfo RecordInfo `json:"recordInfo"`
}

type FastDataJson struct {
	DataList []FastData `json:"dataList"`
	LastId   int        `json:"lastId"`
}

func (f *FastDataJson) SaveToJson(filePath string) error {
	data, err := json.Marshal(f)
	if err != nil {
		return err
	}

	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		return err
	}

	return nil
}

func (f *FastDataJson) LoadFromJson(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, f)
	if err != nil {
		return err
	}

	return nil
}

func (f *FastDataJson) GetInfoForIp(ip string) (FastData, bool) {
	for _, data := range f.DataList {
		if data.RecordInfo.RecordContent == ip {
			return data, true
		}
	}
	return FastData{}, false
}

func (f *FastDataJson) GetInfoForToken(token string) (*FastData, bool) {
	for i, data := range f.DataList {
		if data.Token == token {
			return &f.DataList[i], true
		}
	}
	return &FastData{}, false
}

func GetFastData(filePath string) (FastDataJson, error) {
	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// 使用 os.Create 创建文件，如果文件已存在会清空文件内容
		file, err := os.Create(filePath)
		if err != nil {
			fmt.Println("Error creating file:", err)
		}
		defer file.Close() // 确保文件在函数结束时关闭

		// 写入内容到文件
		_, err = file.WriteString("{\"lastId\": " + strconv.Itoa(AccountConfig.FastConfig.StartId) + "}")
		if err != nil {
			fmt.Println("Error writing to file:", err)
		}
	}
	var fastDataJson FastDataJson
	err := fastDataJson.LoadFromJson(filePath)
	if err != nil {
		return fastDataJson, err
	}

	return fastDataJson, nil
}
