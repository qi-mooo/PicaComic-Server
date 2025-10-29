package services

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif" // 支持 GIF 格式
	"image/jpeg"
	"image/png"
	"os"
	"strconv"
)

// JM 图片反混淆 - 转换自客户端算法

func getSegmentationNum(epsId, scrambleID, pictureName string) int {
	scrambleId, _ := strconv.Atoi(scrambleID)
	epsID, _ := strconv.Atoi(epsId)
	num := 0

	if epsID < scrambleId {
		num = 0
	} else if epsID < 268850 {
		num = 10
	} else if epsID > 421926 {
		// 计算 MD5 hash
		hashStr := fmt.Sprintf("%d%s", epsID, pictureName)
		hash := md5.Sum([]byte(hashStr))
		hashHex := hex.EncodeToString(hash[:])
		// 获取最后一个字符的 ASCII 值
		charCode := int(hashHex[len(hashHex)-1])
		remainder := charCode % 8
		num = remainder*2 + 2
	} else {
		// 计算 MD5 hash
		hashStr := fmt.Sprintf("%d%s", epsID, pictureName)
		hash := md5.Sum([]byte(hashStr))
		hashHex := hex.EncodeToString(hash[:])
		// 获取最后一个字符的 ASCII 值
		charCode := int(hashHex[len(hashHex)-1])
		remainder := charCode % 10
		num = remainder*2 + 2
	}

	return num
}

// DescrambleJmImage 对 JM 图片进行反混淆
func DescrambleJmImage(inputPath, epsId, scrambleId, bookId string) error {
	// 计算分割数
	num := getSegmentationNum(epsId, scrambleId, bookId)
	
	fmt.Printf("[JM反混淆] epsId=%s, scrambleId=%s, bookId=%s, num=%d\n", epsId, scrambleId, bookId, num)
	
	if num <= 1 {
		// 不需要处理
		fmt.Printf("[JM反混淆] 无需反混淆 (num=%d)\n", num)
		return nil
	}

	// 读取图片文件内容
	fileData, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}

	// 检查文件是否为空或太小
	if len(fileData) < 100 {
		return fmt.Errorf("文件太小: %d bytes", len(fileData))
	}

	// 自动检测并解码图片（不依赖扩展名）
	img, format, err := image.Decode(bytes.NewReader(fileData))
	if err != nil {
		return fmt.Errorf("解码图片失败 (format: %s): %w", format, err)
	}
	
	fmt.Printf("[JM反混淆] 检测到图片格式: %s, 尺寸: %dx%d\n", format, img.Bounds().Dx(), img.Bounds().Dy())

	// 获取图片尺寸
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// 计算每个切片的高度
	blockSize := height / num
	remainder := height % num

	// 创建切片信息
	type Block struct {
		start int
		end   int
	}
	blocks := make([]Block, num)

	for i := 0; i < num; i++ {
		start := i * blockSize
		end := start + blockSize
		if i == num-1 {
			end += remainder // 最后一个块包含余数
		}
		blocks[i] = Block{start: start, end: end}
		fmt.Printf("[JM反混淆] 块 %d: start=%d, end=%d, height=%d\n", i, start, end, end-start)
	}

	// 创建新图片
	newImg := image.NewRGBA(bounds)

	// 反向排列切片（从最后一个块开始）
	y := 0
	for i := num - 1; i >= 0; i-- {
		block := blocks[i]
		currBlockHeight := block.end - block.start
		
		fmt.Printf("[JM反混淆] 复制块 %d (src: %d-%d) -> (dst: %d-%d)\n", 
			i, block.start, block.end, y, y+currBlockHeight)

		// 复制像素
		for srcY := block.start; srcY < block.end; srcY++ {
			for x := 0; x < width; x++ {
				newImg.Set(x, y, img.At(x, srcY))
			}
			y++
		}
	}

	// 保存处理后的图片，根据原始格式选择编码器
	outFile, err := os.Create(inputPath)
	if err != nil {
		return fmt.Errorf("创建输出文件失败: %w", err)
	}
	defer outFile.Close()

	// 根据检测到的格式保存
	switch format {
	case "jpeg", "jpg":
		err = jpeg.Encode(outFile, newImg, &jpeg.Options{Quality: 95})
	case "png":
		err = png.Encode(outFile, newImg)
	default:
		// 默认使用 JPEG
		err = jpeg.Encode(outFile, newImg, &jpeg.Options{Quality: 95})
	}

	if err != nil {
		return fmt.Errorf("编码图片失败: %w", err)
	}

	fmt.Printf("[JM反混淆] 图片已保存 (格式: %s)\n", format)
	return nil
}
