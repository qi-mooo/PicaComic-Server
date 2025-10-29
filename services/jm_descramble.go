package services

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif" // 支持 GIF 格式
	"image/jpeg"
	"image/png"
	"os"
	"strconv"
)

// JM 图片反混淆

func getSegmentationNum(epsId, scrambleID, pictureName string) int {
	scrambleId, _ := strconv.Atoi(scrambleID)
	epsID, _ := strconv.Atoi(epsId)

	if epsID < scrambleId {
		return 0
	} else if epsID < 268850 {
		return 10
	} else if epsID > 421926 {
		hashStr := fmt.Sprintf("%d%s", epsID, pictureName)
		hash := 0
		for _, char := range hashStr {
			hash = int(char) + ((hash << 5) - hash)
		}
		return hash % 10
	}
	return 0
}

// DescrambleJmImage 对 JM 图片进行反混淆
func DescrambleJmImage(inputPath, epsId, scrambleId, bookId string) error {
	// 计算分割数
	num := getSegmentationNum(epsId, scrambleId, bookId)

	if num <= 1 {
		// 不需要处理
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
	remainder := height % num
	copyHeight := height / num

	// 创建新图片
	newImg := image.NewRGBA(bounds)

	// 重新排列切片
	for i := 0; i < num; i++ {
		copyY := copyHeight * i
		pasteY := height - copyHeight*(i+1)

		if i < remainder {
			pasteY -= remainder - i
		} else {
			pasteY -= remainder
		}

		// 复制像素
		for y := 0; y < copyHeight; y++ {
			for x := 0; x < width; x++ {
				srcY := copyY + y
				dstY := pasteY + y
				if srcY < height && dstY >= 0 && dstY < height {
					newImg.Set(x, dstY, img.At(x, srcY))
				}
			}
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
