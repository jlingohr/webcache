package webcache

import (
	"fmt"
	"log"
)

func BytesToMegabyte(b int) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "kMGTPE"[exp])
}

func CacheStatus(current int, max int) {
	log.Print(fmt.Sprintf("CAPACITY - %s of %s", BytesToMegabyte(current), BytesToMegabyte(max)))
}
