package services

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

	// 读取图片
	file, err := os.Open(inputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// 解码图片
	var img image.Image
	ext := strings.ToLower(filepath.Ext(inputPath))
	switch ext {
	case ".jpg", ".jpeg":
		img, err = jpeg.Decode(file)
	case ".png":
		img, err = png.Decode(file)
	default:
		img, _, err = image.Decode(file)
	}
	
	if err != nil {
		return fmt.Errorf("解码图片失败: %w", err)
	}
	file.Close()

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
		pasteY := height - copyHeight * (i + 1)
		
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

	// 保存处理后的图片
	outFile, err := os.Create(inputPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	switch ext {
	case ".jpg", ".jpeg":
		return jpeg.Encode(outFile, newImg, &jpeg.Options{Quality: 95})
	case ".png":
		return png.Encode(outFile, newImg)
	default:
		return jpeg.Encode(outFile, newImg, &jpeg.Options{Quality: 95})
	}
}
