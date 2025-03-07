package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/alibabacloud-go/tea/tea"
	"net"
	"reflect"
	"strings"
	"time"
)

func SetRequestFieldsWithTag(source interface{}, target interface{}, DNSFromTag string) {
	// 获取 source 和 target 的实际值（解引用指针）
	sourceValue := reflect.ValueOf(source).Elem()
	targetValue := reflect.ValueOf(target).Elem()

	// 遍历 source 的所有字段
	for i := 0; i < sourceValue.NumField(); i++ {
		field := sourceValue.Field(i)
		// 获取字段的 DNSFrom 标签
		fieldTag := sourceValue.Type().Field(i).Tag.Get(DNSFromTag) // 假设 DNSFromTag 是 "DNSFrom"
		if fieldTag == "" {
			continue // 如果标签为空，跳过该字段
		}

		// 根据标签名称找到 target 中对应的字段
		targetField := targetValue.FieldByName(fieldTag)
		if !targetField.IsValid() || !targetField.CanSet() {
			continue // 如果字段无效或不可设置，跳过
		}

		// 根据字段类型进行赋值
		switch field.Kind() {
		case reflect.Int64:
			if field.Int() != 0 {
				targetField.Set(reflect.ValueOf(tea.Int64(field.Int())))
			}
		case reflect.Int32:
			if field.Int() != 0 {
				targetField.Set(reflect.ValueOf(tea.Int32(int32(field.Int()))))
			}
		case reflect.String:
			if field.String() != "" {
				targetField.Set(reflect.ValueOf(tea.String(field.String())))
			}
		case reflect.Uint64:
			if field.Uint() != 0 {
				targetField.Set(reflect.ValueOf(tea.Uint64(field.Uint())))
			}

		default:
			targetField.Set(field) // 其他类型直接赋值
		}
	}
}

// HashStringWithCurrentTime 接受一个字符串并使用当前时间对其进行哈希处理，返回哈希字符串
func HashStringWithCurrentTime(input string) string {
	// 获取当前时间
	currentTime := time.Now().String()

	// 将输入字符串和当前时间拼接
	data := input + currentTime

	// 计算 SHA-256 哈希值
	hash := sha256.Sum256([]byte(data))

	// 将哈希值转换为十六进制字符串
	hashString := hex.EncodeToString(hash[:])

	return hashString
}

// HashString 接受一个字符串对其进行哈希处理，返回前5位哈希字符串
func HashString(input string) string {
	// 计算 SHA-256 哈希值
	hash := sha256.Sum256([]byte(input))

	// 将哈希值转换为十六进制字符串
	hashString := hex.EncodeToString(hash[:])

	// 返回前5位哈希字符串
	return hashString[:5]
}

// IsCNAMEEqual 判断第一个域名的 CNAME 值是否等于第二个域名
func IsCNAMEEqual(domain1, domain2 string) bool {
	// 解析 domain1 的 CNAME 记录
	cname, err := net.LookupCNAME(domain1)
	if err != nil {
		// 如果解析失败，返回 false
		return false
	}
	cname = strings.ReplaceAll(cname, ".", "")
	domain2 = strings.ReplaceAll(domain2, ".", "")
	// 将 CNAME 和 domain2 转换为小写后比较，忽略大小写
	return strings.ToLower(cname) == strings.ToLower(domain2)
}
