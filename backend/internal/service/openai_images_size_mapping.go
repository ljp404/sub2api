package service

import (
	"fmt"
	"maps"
	"math/big"
	"slices"
	"strconv"
	"strings"
)

const imageSizeMappingMaxSide = 65536

func imageSizeMappingEnabled(rule *ParameterPricing) bool {
	for _, factor := range rule.Factors {
		if factor.Key == "resolution" || factor.Key == "aspect_ratio" {
			return true
		}
	}
	return false
}

func parseImageSizePair(value, separator string) (int64, int64, bool) {
	left, right, ok := strings.Cut(value, separator)
	if !ok || left == "" || right == "" {
		return 0, 0, false
	}
	for _, part := range []string{left, right} {
		for _, digit := range part {
			if digit < '0' || digit > '9' {
				return 0, 0, false
			}
		}
	}
	w, e1 := strconv.ParseInt(left, 10, 64)
	h, e2 := strconv.ParseInt(right, 10, 64)
	return w, h, e1 == nil && e2 == nil && w > 0 && h > 0 &&
		w <= imageSizeMappingMaxSide && h <= imageSizeMappingMaxSide
}

func imageSizeMappedDimensions(req *OpenAIImagesRequest, rule *ParameterPricing) (map[string]string, error) {
	dimensions := imagePriceDimensions(req)
	if !imageSizeMappingEnabled(rule) {
		return dimensions, nil
	}
	size := normalizeParameterValue(req.Size)
	if size == "" {
		return nil, fmt.Errorf("当前计费模式为：图片（参数），缺少必填参数 size，请传 auto 或 宽x高（例如 1920x1080）")
	}
	if req.InvalidPriceDimensions["size"] {
		return nil, fmt.Errorf("当前计费模式为：图片（参数），参数 size 必须是唯一的字符串")
	}
	var w, h int64
	if size != "auto" {
		var valid bool
		w, h, valid = parseImageSizePair(size, "x")
		if !valid {
			return nil, fmt.Errorf("当前计费模式为：图片（参数），size %q 无效，请传 auto 或 宽x高，宽高须为 1 至 %d 的整数", size, imageSizeMappingMaxSide)
		}
	}
	// Client-supplied aliases must not override the values used for forwarding.
	delete(dimensions, "resolution")
	delete(dimensions, "aspect_ratio")
	delete(dimensions, "image_size")
	for _, factor := range rule.Factors {
		if factor.Key != "resolution" && factor.Key != "aspect_ratio" {
			continue
		}
		value := "auto"
		if factor.Key == "resolution" {
			value = "2k"
		}
		if size == "auto" {
			if _, ok := factor.Values[value]; !ok {
				return nil, fmt.Errorf("当前计费模式为：图片（参数），size=auto 需要当前模型配置 %s=%s", factor.Key, value)
			}
		} else {
			var err error
			if factor.Key == "resolution" {
				longSide := w
				if h > longSide {
					longSide = h
				}
				value, err = nearestImageResolution(longSide, factor.Values)
			} else {
				value, err = nearestImageAspectRatio(w, h, factor.Values)
			}
			if err != nil {
				return nil, fmt.Errorf("当前计费模式为：图片（参数），size=%s 转换失败：%w", size, err)
			}
		}
		dimensions[factor.Key] = value
	}
	return dimensions, nil
}

func nearestImageResolution(longSide int64, options map[string]string) (string, error) {
	best, bestSide, bestDistance := "", int64(0), int64(0)
	for _, option := range slices.Sorted(maps.Keys(options)) {
		k, err := strconv.ParseInt(strings.TrimSuffix(option, "k"), 10, 64)
		if !strings.HasSuffix(option, "k") || err != nil || k <= 0 || k > imageSizeMappingMaxSide/1024 {
			return "", fmt.Errorf("resolution 配置值 %q 不是有效的整数 k 档位", option)
		}
		side := k * 1024
		distance := longSide - side
		if distance < 0 {
			distance = -distance
		}
		if best == "" || distance < bestDistance || (distance == bestDistance && side > bestSide) {
			best, bestSide, bestDistance = option, side, distance
		}
	}
	if best == "" {
		return "", fmt.Errorf("resolution 没有可匹配的档位")
	}
	return best, nil
}

func nearestImageAspectRatio(w, h int64, options map[string]string) (string, error) {
	var bestScore, bestRatio *big.Rat
	best := ""
	for _, option := range slices.Sorted(maps.Keys(options)) {
		if option == "auto" {
			continue
		}
		a, b, valid := parseImageSizePair(option, ":")
		if !valid {
			return "", fmt.Errorf("aspect_ratio 配置值 %q 不是有效的宽:高比例", option)
		}
		if (w > h) != (a > b) || (w < h) != (a < b) {
			continue
		}
		// Minimizing max(r/c, c/r) is equivalent to minimizing abs(log(r/c)).
		// Exact fractions also make equal-distance tie breaking deterministic.
		numerator, denominator := w*b, h*a
		if numerator < denominator {
			numerator, denominator = denominator, numerator
		}
		score := big.NewRat(numerator, denominator)
		ratio := big.NewRat(a, b)
		if best == "" || score.Cmp(bestScore) < 0 ||
			(score.Cmp(bestScore) == 0 && ratio.Cmp(bestRatio) < 0) {
			best, bestScore, bestRatio = option, score, ratio
		}
	}
	if best == "" {
		return "", fmt.Errorf("aspect_ratio 没有与请求宽高同方向的已配置比例")
	}
	return best, nil
}
