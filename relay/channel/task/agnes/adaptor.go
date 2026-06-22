package agnes

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/tidwall/sjson"
)

type agnesVideoRequest struct {
	Model     string      `json:"model"`
	Prompt    string      `json:"prompt"`
	Image     interface{} `json:"image,omitempty"`
	Height    int         `json:"height,omitempty"`
	Width     int         `json:"width,omitempty"`
	NumFrames int         `json:"num_frames,omitempty"`
	FrameRate float64     `json:"frame_rate,omitempty"`
	Seed      int         `json:"seed,omitempty"`
	NegPrompt string      `json:"negative_prompt,omitempty"`
	ExtraBody *extraBody  `json:"extra_body,omitempty"`
}

type extraBody struct {
	Image []string `json:"image,omitempty"`
	Mode  string   `json:"mode,omitempty"`
}

type agnesVideoResponse struct {
	ID        string `json:"id"`
	TaskID    string `json:"task_id,omitempty"`
	VideoID   string `json:"video_id,omitempty"`
	Object    string `json:"object"`
	Model     string `json:"model"`
	Status    string `json:"status"`
	Progress  int    `json:"progress"`
	CreatedAt int64  `json:"created_at"`
	Seconds   string `json:"seconds,omitempty"`
	Size      string `json:"size,omitempty"`
	VideoURL  string `json:"video_url,omitempty"`
	URL       string `json:"url,omitempty"`
	Error     *struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	apiKey      string
	baseURL     string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = info.ChannelBaseUrl
	a.apiKey = info.ApiKey
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) (taskErr *dto.TaskError) {
	return relaycommon.ValidateMultipartDirect(c, info)
}

func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}
	seconds, _ := strconv.Atoi(req.Seconds)
	if seconds == 0 {
		seconds = req.Duration
	}
	if seconds <= 0 {
		seconds = 5
	}
	return map[string]float64{
		"seconds": float64(seconds),
		"size":    1,
	}
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return fmt.Sprintf("%s/v1/videos", a.baseURL), nil
}

func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")
	return nil
}

// formValue helper for multipart.Form.Value which is map[string][]string
func formValue(form map[string][]string, key string) string {
	vals, ok := form[key]
	if !ok || len(vals) == 0 {
		return ""
	}
	return vals[0]
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	contentType := c.GetHeader("Content-Type")

	var prompt string
	var imageStr string
	var seconds int
	var size string

	if strings.Contains(contentType, "multipart/form-data") {
		formData, err := common.ParseMultipartFormReusable(c)
		if err != nil {
			return nil, errors.Wrap(err, "parse multipart failed")
		}
		prompt = formValue(formData.Value, "prompt")
		seconds, _ = strconv.Atoi(formValue(formData.Value, "seconds"))
		size = formValue(formData.Value, "size")
		if img := formValue(formData.Value, "image"); img != "" {
			imageStr = img
		}
	} else {
		// JSON input
		storage, err := common.GetBodyStorage(c)
		if err != nil {
			return nil, errors.Wrap(err, "get body failed")
		}
		cachedBody, err := storage.Bytes()
		if err != nil {
			return nil, errors.Wrap(err, "read body failed")
		}
		var bodyMap map[string]interface{}
		if err := common.Unmarshal(cachedBody, &bodyMap); err == nil {
			if p, ok := bodyMap["prompt"].(string); ok {
				prompt = p
			}
			if s, ok := bodyMap["seconds"].(string); ok {
				seconds, _ = strconv.Atoi(s)
			}
			if s, ok := bodyMap["size"].(string); ok {
				size = s
			}
			if img, ok := bodyMap["image"].(string); ok {
				imageStr = img
			}
		}
	}

	agnesReq := agnesVideoRequest{
		Model:  info.UpstreamModelName,
		Prompt: prompt,
	}

	// Parse size
	if size != "" {
		parts := strings.Split(size, "x")
		if len(parts) == 2 {
			agnesReq.Width, _ = strconv.Atoi(parts[0])
			agnesReq.Height, _ = strconv.Atoi(parts[1])
		}
	}
	if agnesReq.Width == 0 {
		agnesReq.Width = 1152
	}
	if agnesReq.Height == 0 {
		agnesReq.Height = 768
	}

	// Frames: 8n+1 formula, max 441
	if seconds <= 0 {
		seconds = 5
	}
	agnesReq.FrameRate = 24
	agnesReq.NumFrames = seconds*24 + 1
	if agnesReq.NumFrames > 441 {
		agnesReq.NumFrames = 441
	}

	if imageStr != "" {
		agnesReq.Image = imageStr
	}

	body, err := common.Marshal(agnesReq)
	if err != nil {
		return nil, errors.Wrap(err, "marshal agnes request failed")
	}
	return bytes.NewReader(body), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}
	_ = resp.Body.Close()

	var dResp agnesVideoResponse
	if err := common.Unmarshal(responseBody, &dResp); err != nil {
		taskErr = service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
		return
	}

	upstreamID := dResp.VideoID
	if upstreamID == "" {
		upstreamID = dResp.TaskID
	}
	if upstreamID == "" {
		upstreamID = dResp.ID
	}
	if upstreamID == "" {
		taskErr = service.TaskErrorWrapper(fmt.Errorf("task_id is empty"), "invalid_response", http.StatusInternalServerError)
		return
	}

	dResp.ID = info.PublicTaskID
	dResp.TaskID = info.PublicTaskID
	c.JSON(http.StatusOK, dResp)
	return upstreamID, responseBody, nil
}

func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid task_id")
	}
	uri := fmt.Sprintf("%s/agnesapi?video_id=%s", baseUrl, taskID)
	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var res agnesVideoResponse
	if err := common.Unmarshal(respBody, &res); err != nil {
		return nil, errors.Wrap(err, "unmarshal task result failed")
	}
	taskResult := relaycommon.TaskInfo{Code: 0}
	switch res.Status {
	case "queued", "pending":
		taskResult.Status = model.TaskStatusQueued
	case "running", "processing", "in_progress":
		taskResult.Status = model.TaskStatusInProgress
	case "succeeded", "completed":
		taskResult.Status = model.TaskStatusSuccess
		if res.VideoURL != "" {
			taskResult.RemoteUrl = res.VideoURL
		} else if res.URL != "" {
			taskResult.RemoteUrl = res.URL
		}
	case "failed", "cancelled", "expired":
		taskResult.Status = model.TaskStatusFailure
		if res.Error != nil {
			taskResult.Reason = res.Error.Message
		} else {
			taskResult.Reason = "task failed"
		}
	}
	if res.Progress > 0 && res.Progress < 100 {
		taskResult.Progress = fmt.Sprintf("%d%%", res.Progress)
	}
	return &taskResult, nil
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(task *model.Task) ([]byte, error) {
	data := task.Data
	var err error
	if data, err = sjson.SetBytes(data, "id", task.TaskID); err != nil {
		return nil, errors.Wrap(err, "set id failed")
	}
	return data, nil
}

func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}
