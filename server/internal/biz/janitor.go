package biz

// 孤儿图片清理（janitor，定时任务）。
//
// 为什么会有孤儿：本系统是两步上传（先 POST /uploads 拿 URL，再随物品 JSON 提交），
// 用户上传后放弃提交、或编辑时换图，都会留下无引用文件。文件生命周期与业务数据
// 脱钩是两步上传的固有代价，只能靠对账清理。
//
// 策略（对账 + 宽限期）：
//   1. 引用集合 = biz_item.image 全量（含软删行——软删可恢复，图片保守保留）
//   2. 扫描 uploads 目录，不在引用集合里、且 mtime 距今超过宽限期的文件删除
//   3. 宽限期（默认 24h）覆盖"传了图还在填表单"的场景——刚上传的文件不删
//
// 已知的极小竞态：清理过程中用户恰好提交了引用该图的物品。窗口毫秒级且宽限期
// 把新文件排除在外，实际风险可忽略；严格做法是"先标记、下轮再删"两阶段清理。

import (
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

// StartJanitor 启动后台清理协程：每 interval 执行一轮，无引用且超过 grace 的文件删除。
// main 里调用一次即可；time.Ticker 常驻，进程退出随之结束。
func (h *Handler) StartJanitor(interval, grace time.Duration) {
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for range t.C {
			if n := h.cleanOrphans(grace); n > 0 {
				h.auditRaw("UPLOAD_CLEAN", "uploads/", "清理无引用图片文件 "+strconv.Itoa(n)+" 个")
			}
		}
	}()
}

// cleanOrphans 执行一轮对账清理，返回删除的文件数。
func (h *Handler) cleanOrphans(grace time.Duration) int {
	// 1. 引用集合：image 存的是 "/uploads/xxx.jpg"，取 basename 归一化比对
	refs := map[string]bool{}
	rows, err := h.db.Query(`SELECT image FROM biz_item WHERE image IS NOT NULL`)
	if err != nil {
		return 0
	}
	defer rows.Close()
	for rows.Next() {
		var img string
		if rows.Scan(&img) == nil && img != "" {
			refs[path.Base(strings.ReplaceAll(img, "\\", "/"))] = true
		}
	}

	// 2. 扫描目录
	entries, err := os.ReadDir(h.dir)
	if err != nil {
		return 0
	}

	removed := 0
	cutoff := time.Now().Add(-grace)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if refs[e.Name()] {
			continue // 活文件
		}
		fi, err := e.Info()
		if err != nil || fi.ModTime().After(cutoff) {
			continue // 宽限期内：可能正在编辑表单
		}
		if os.Remove(path.Join(h.dir, e.Name())) == nil {
			removed++
		}
		// 删除失败（如 Windows 上文件被占用）跳过，下轮再试
	}
	return removed
}

// auditRaw 无请求上下文的审计（后台任务用，无 IP/UA）
func (h *Handler) auditRaw(action, target, detail string) {
	h.authS.AuditLog(nil, "-", action, target, detail, "-", "janitor")
}
