package routes

import (
	"EverythingSuckz/fsb/config"
	"EverythingSuckz/fsb/internal/bot"
	"EverythingSuckz/fsb/internal/stream"
	"EverythingSuckz/fsb/internal/types"
	"EverythingSuckz/fsb/internal/utils"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gotd/td/tg"
	range_parser "github.com/quantumsheep/range-parser"
	"go.uber.org/zap"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

var log *zap.Logger

// Redis 客户端，在 LoadHome 里按配置初始化（REDIS_ADDR / REDIS_PASSWORD / REDIS_DB）
var rdb *redis.Client

func (e *allRoutes) LoadHome(r *Route) {
	log = e.log.Named("Stream")
	defer log.Info("Loaded stream route")
	rdb = redis.NewClient(&redis.Options{
		Addr:     config.ValueOf.RedisAddr,
		Password: config.ValueOf.RedisPassword,
		DB:       config.ValueOf.RedisDB,
	})
	r.Engine.GET("/stream/:messageID", getStreamRoute)
	r.Engine.GET("/v/:filename", StreamByFileName)
}

func getStreamRoute(ctx *gin.Context) {
	w := ctx.Writer
	r := ctx.Request

	messageIDParm := ctx.Param("messageID")
	messageID, err := strconv.Atoi(messageIDParm)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	authHash := ctx.Query("hash")
	if authHash == "" {
		http.Error(w, "missing hash param", http.StatusBadRequest)
		return
	}

	file, worker, err := utils.TimeFuncWithResult2(log, "FileFromMessage", func() (*types.File, *bot.Worker, error) {
		return fetchFile(ctx, messageID)
	})
	if err != nil {
		http.Error(w, err.Error(), fileErrorStatus(err, http.StatusBadRequest))
		return
	}

	expectedHash := utils.PackFile(
		file.FileName,
		file.FileSize,
		file.MimeType,
		file.ID,
	)
	if !utils.CheckHash(authHash, expectedHash) {
		http.Error(w, "invalid hash", http.StatusBadRequest)
		return
	}

	title := ctx.Query("title")
	var filename string
	disposition := "inline"
	if title != "" {
		disposition = "attachment"
		title = strings.ReplaceAll(title, "\"", "_")
		title = strings.ReplaceAll(title, "\r", "")
		title = strings.ReplaceAll(title, "\n", "")

		ext := filepath.Ext(file.FileName)
		if ext != "" && !strings.HasSuffix(strings.ToLower(title), strings.ToLower(ext)) {
			filename = title + ext
		} else {
			filename = title
		}
	} else {
		filename = file.FileName
		if ctx.Query("d") == "true" {
			disposition = "attachment"
		}
	}

	// for photo messages
	if file.FileSize == 0 {
		res, err := worker.Client.API().UploadGetFile(ctx, &tg.UploadGetFileRequest{
			Location: file.Location,
			Offset:   0,
			Limit:    1024 * 1024,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		result, ok := res.(*tg.UploadFile)
		if !ok {
			http.Error(w, "unexpected response", http.StatusInternalServerError)
			return
		}
		fileBytes := result.GetBytes()
		ctx.Header("Content-Disposition", fmt.Sprintf("%s; filename=\"%s\"", disposition, filename))
		if r.Method != "HEAD" {
			ctx.Data(http.StatusOK, file.MimeType, fileBytes)
		}
		return
	}

	ctx.Header("Accept-Ranges", "bytes")
	var start, end int64
	rangeHeader := r.Header.Get("Range")

	if rangeHeader == "" {
		start = 0
		end = file.FileSize - 1
		w.WriteHeader(http.StatusOK)
	} else {
		ranges, err := range_parser.Parse(file.FileSize, r.Header.Get("Range"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		start = ranges[0].Start
		end = ranges[0].End
		ctx.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, file.FileSize))
		log.Info("Content-Range", zap.Int64("start", start), zap.Int64("end", end), zap.Int64("fileSize", file.FileSize))
		w.WriteHeader(http.StatusPartialContent)
	}

	contentLength := end - start + 1
	mimeType := file.MimeType

	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	ctx.Header("Content-Type", mimeType)
	ctx.Header("Content-Length", strconv.FormatInt(contentLength, 10))

	ctx.Header("Content-Disposition", fmt.Sprintf("%s; filename=\"%s\"", disposition, filename))

	if r.Method != "HEAD" {
		pipe, err := stream.NewStreamPipe(ctx, worker.Client, file.Location, start, end, log)
		if err != nil {
			log.Error("Failed to create stream pipe", zap.Error(err))
			return
		}
		defer pipe.Close()
		if _, err := io.CopyN(w, pipe, contentLength); err != nil {
			if !utils.IsClientDisconnectError(err) {
				log.Error("Error while copying stream", zap.Error(err))
			}
		}
	}
}

// StreamByFileName 处理 /v/:filename 的请求
func StreamByFileName(c *gin.Context) {
	// 1. 获取 URL 中的文件名，例如 "S0107.mp4" 或 "S0107"
	filename := c.Param("filename")

	// 2. 去除后缀名，拿到纯编号 "S0107"，以匹配 Redis 中的 Key
	nameOnly := strings.TrimSuffix(filename, filepath.Ext(filename))
	redisKey := "vid:" + nameOnly

	// 3. 从 Redis 查询对应的 Message ID
	msgIDStr, err := rdb.Get(c.Request.Context(), redisKey).Result()
	if err == redis.Nil {
		c.String(http.StatusNotFound, "Video mapping not found in Redis")
		return
	} else if err != nil {
		c.String(http.StatusInternalServerError, "Redis query error")
		return
	}

	messageID, err := strconv.Atoi(msgIDStr)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid message ID in Redis")
		return
	}

	// 4. 获取文件信息以生成正确的 Hash 校验参数
	file, _, err := fetchFile(c, messageID)
	if err != nil {
		c.String(fileErrorStatus(err, http.StatusInternalServerError), "Failed to fetch file info from Telegram: "+err.Error())
		return
	}

	expectedHash := utils.PackFile(
		file.FileName,
		file.FileSize,
		file.MimeType,
		file.ID,
	)
	hash := utils.GetShortHash(expectedHash)

	// 5. 核心：直接 302 重定向到带 Hash 校验的原生播放路由
	q := c.Request.URL.Query()
	q.Set("hash", hash)
	targetURL := fmt.Sprintf("/stream/%d?%s", messageID, q.Encode())
	c.Redirect(http.StatusFound, targetURL)
}

// fetchFile 取文件信息。遇到 Telegram 连接类错误（如 CONNECTION_NOT_INITED）时，
// 换下一个 worker 重试，直到所有 worker 都试过；其他错误直接返回。
func fetchFile(ctx context.Context, messageID int) (*types.File, *bot.Worker, error) {
	attempts := len(bot.Workers.Bots)
	if attempts == 0 {
		return nil, nil, errors.New("no bot workers available")
	}
	var lastErr error
	for i := 0; i < attempts; i++ {
		worker := bot.GetNextWorker()
		file, err := utils.FileFromMessage(ctx, worker.Client, messageID)
		if err == nil {
			return file, worker, nil
		}
		lastErr = err
		if !utils.IsTGConnectionError(err) || ctx.Err() != nil {
			break
		}
		log.Warn("Telegram connection error, trying next worker",
			zap.Int("workerID", worker.ID), zap.Int("messageID", messageID), zap.Error(err))
	}
	return nil, nil, lastErr
}

// fileErrorStatus 把取文件信息的错误映射成 HTTP 状态码。
func fileErrorStatus(err error, fallback int) int {
	switch {
	case errors.Is(err, utils.ErrFileDeleted):
		return http.StatusNotFound
	case utils.IsTGConnectionError(err):
		return http.StatusServiceUnavailable
	default:
		return fallback
	}
}
