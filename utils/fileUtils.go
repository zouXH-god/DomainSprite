package utils

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ZipFolder 压缩指定文件夹，如果zip已存在则直接返回路径
func ZipFolder(folderPath string) (string, error) {
	// 检查文件夹是否存在
	info, err := os.Stat(folderPath)
	if err != nil {
		return "", fmt.Errorf("文件夹不存在或无法访问: %v", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("提供的路径不是文件夹")
	}

	// 构造zip文件路径（与文件夹同目录，同名.zip）
	zipPath := filepath.Join(filepath.Dir(folderPath), filepath.Base(folderPath)+".zip")

	// 检查zip文件是否已存在
	if _, err := os.Stat(zipPath); err == nil {
		return zipPath, nil
	}

	// 创建zip文件
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return "", fmt.Errorf("无法创建zip文件: %v", err)
	}
	defer zipFile.Close()

	// 创建zip writer
	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// 遍历文件夹并添加文件到zip
	err = filepath.Walk(folderPath, func(filePath string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过目录本身（只添加目录中的内容）
		if filePath == folderPath {
			return nil
		}

		// 创建文件头信息
		header, err := zip.FileInfoHeader(fileInfo)
		if err != nil {
			return err
		}

		// 确保路径是相对路径（相对于源文件夹）
		relPath, err := filepath.Rel(folderPath, filePath)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath) // 确保使用斜杠分隔符

		// 如果是目录，需要在名称末尾添加斜杠
		if fileInfo.IsDir() {
			header.Name += "/"
		} else {
			// 设置压缩方法
			header.Method = zip.Deflate
		}

		// 写入文件头
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		// 如果是目录，没有内容需要写入
		if fileInfo.IsDir() {
			return nil
		}

		// 打开文件并写入内容
		file, err := os.Open(filePath)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})

	if err != nil {
		// 如果出错，尝试删除可能不完整的zip文件
		os.Remove(zipPath)
		return "", fmt.Errorf("创建zip过程中出错: %v", err)
	}

	return zipPath, nil
}

// ReadFileContent 读取文件内容并返回字符串
func ReadFileContent(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	// 将字节切片转换为字符串并返回
	return string(content), nil
}
