package sarin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"go.aykhans.me/sarin/internal/types"
)

const (
	captchaPollInterval = 5 * time.Second
	captchaTimeout      = 120 * time.Second
)

var captchaHTTPClient = &http.Client{Timeout: captchaTimeout}

// solveCaptcha creates a task and polls for the result.
// baseURL is the service API base (e.g. "https://api.2captcha.com").
// taskIDIsString controls whether taskId is sent back as a string or number.
// solutionKey is the field name in the solution object that holds the token.
func solveCaptcha(baseURL, apiKey string, task map[string]any, solutionKey string, taskIDIsString bool) (string, error) {
	if apiKey == "" {
		return "", types.ErrCaptchaKeyEmpty
	}

	taskID, err := captchaCreateTask(baseURL, apiKey, task)
	if err != nil {
		return "", err
	}
	return captchaPollResult(baseURL, apiKey, taskID, solutionKey, taskIDIsString)
}

func captchaCreateTask(baseURL, apiKey string, task map[string]any) (string, error) {
	body := map[string]any{
		"clientKey": apiKey,
		"task":      task,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return "", types.NewCaptchaRequestError("createTask", err)
	}

	resp, err := captchaHTTPClient.Post(
		baseURL+"/createTask",
		"application/json",
		bytes.NewReader(data),
	)
	if err != nil {
		return "", types.NewCaptchaRequestError("createTask", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	var result struct {
		ErrorID          int             `json:"errorId"`
		ErrorCode        string          `json:"errorCode"`
		ErrorDescription string          `json:"errorDescription"`
		TaskID           json.RawMessage `json:"taskId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", types.NewCaptchaRequestError("createTask", err)
	}

	if result.ErrorID != 0 {
		return "", types.NewCaptchaAPIError("createTask", result.ErrorCode, result.ErrorDescription)
	}

	// taskId may be a JSON number (2captcha, anti-captcha) or a quoted string (capsolver UUIDs).
	// Strip surrounding quotes if present so we always work with the underlying value.
	return strings.Trim(string(result.TaskID), `"`), nil
}

func captchaPollResult(baseURL, apiKey, taskID, solutionKey string, taskIDIsString bool) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), captchaTimeout)
	defer cancel()

	ticker := time.NewTicker(captchaPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", types.NewCaptchaTimeoutError(taskID)
		case <-ticker.C:
			token, done, err := captchaGetTaskResult(baseURL, apiKey, taskID, solutionKey, taskIDIsString)
			if err != nil {
				return "", err
			}
			if done {
				return token, nil
			}
		}
	}
}

func captchaGetTaskResult(baseURL, apiKey, taskID, solutionKey string, taskIDIsString bool) (string, bool, error) {
	var bodyMap map[string]any
	if taskIDIsString {
		bodyMap = map[string]any{"clientKey": apiKey, "taskId": taskID}
	} else {
		bodyMap = map[string]any{"clientKey": apiKey, "taskId": json.Number(taskID)}
	}

	data, err := json.Marshal(bodyMap)
	if err != nil {
		return "", false, types.NewCaptchaRequestError("getTaskResult", err)
	}

	resp, err := captchaHTTPClient.Post(
		baseURL+"/getTaskResult",
		"application/json",
		bytes.NewReader(data),
	)
	if err != nil {
		return "", false, types.NewCaptchaRequestError("getTaskResult", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	var result struct {
		ErrorID          int            `json:"errorId"`
		ErrorCode        string         `json:"errorCode"`
		ErrorDescription string         `json:"errorDescription"`
		Status           string         `json:"status"`
		Solution         map[string]any `json:"solution"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", false, types.NewCaptchaRequestError("getTaskResult", err)
	}

	if result.ErrorID != 0 {
		return "", false, types.NewCaptchaAPIError("getTaskResult", result.ErrorCode, result.ErrorDescription)
	}

	if result.Status == "processing" || result.Status == "idle" {
		return "", false, nil
	}

	token, ok := result.Solution[solutionKey]
	if !ok {
		return "", false, types.NewCaptchaSolutionKeyError(solutionKey)
	}
	tokenStr, ok := token.(string)
	if !ok {
		return "", false, types.NewCaptchaSolutionKeyError(solutionKey)
	}

	return tokenStr, true, nil
}

// ======================================== 2Captcha ========================================

const twoCaptchaBaseURL = "https://api.2captcha.com"

func twoCaptchaSolveRecaptchaV2(apiKey, websiteURL, websiteKey string) (string, error) {
	return solveCaptcha(twoCaptchaBaseURL, apiKey, map[string]any{
		"type":       "RecaptchaV2TaskProxyless",
		"websiteURL": websiteURL,
		"websiteKey": websiteKey,
	}, "gRecaptchaResponse", false)
}

func twoCaptchaSolveRecaptchaV3(apiKey, websiteURL, websiteKey, pageAction string) (string, error) {
	task := map[string]any{
		"type":       "RecaptchaV3TaskProxyless",
		"websiteURL": websiteURL,
		"websiteKey": websiteKey,
	}
	if pageAction != "" {
		task["pageAction"] = pageAction
	}
	return solveCaptcha(twoCaptchaBaseURL, apiKey, task, "gRecaptchaResponse", false)
}

func twoCaptchaSolveTurnstile(apiKey, websiteURL, websiteKey, cData string) (string, error) {
	task := map[string]any{
		"type":       "TurnstileTaskProxyless",
		"websiteURL": websiteURL,
		"websiteKey": websiteKey,
	}
	if cData != "" {
		task["data"] = cData
	}
	return solveCaptcha(twoCaptchaBaseURL, apiKey, task, "token", false)
}

// ======================================== Anti-Captcha ========================================

const antiCaptchaBaseURL = "https://api.anti-captcha.com"

func antiCaptchaSolveRecaptchaV2(apiKey, websiteURL, websiteKey string) (string, error) {
	return solveCaptcha(antiCaptchaBaseURL, apiKey, map[string]any{
		"type":       "RecaptchaV2TaskProxyless",
		"websiteURL": websiteURL,
		"websiteKey": websiteKey,
	}, "gRecaptchaResponse", false)
}

func antiCaptchaSolveRecaptchaV3(apiKey, websiteURL, websiteKey, pageAction string) (string, error) {
	// Anti-Captcha requires minScore for reCAPTCHA v3. 0.3 is the loosest threshold.
	task := map[string]any{
		"type":       "RecaptchaV3TaskProxyless",
		"websiteURL": websiteURL,
		"websiteKey": websiteKey,
		"minScore":   0.3,
	}
	if pageAction != "" {
		task["pageAction"] = pageAction
	}
	return solveCaptcha(antiCaptchaBaseURL, apiKey, task, "gRecaptchaResponse", false)
}

func antiCaptchaSolveHCaptcha(apiKey, websiteURL, websiteKey string) (string, error) {
	// Anti-Captcha returns hCaptcha tokens under "gRecaptchaResponse" (not "token").
	return solveCaptcha(antiCaptchaBaseURL, apiKey, map[string]any{
		"type":       "HCaptchaTaskProxyless",
		"websiteURL": websiteURL,
		"websiteKey": websiteKey,
	}, "gRecaptchaResponse", false)
}

func antiCaptchaSolveTurnstile(apiKey, websiteURL, websiteKey, cData string) (string, error) {
	task := map[string]any{
		"type":       "TurnstileTaskProxyless",
		"websiteURL": websiteURL,
		"websiteKey": websiteKey,
	}
	if cData != "" {
		task["cData"] = cData
	}
	return solveCaptcha(antiCaptchaBaseURL, apiKey, task, "token", false)
}

// ======================================== CapSolver ========================================

const capSolverBaseURL = "https://api.capsolver.com"

func capSolverSolveRecaptchaV2(apiKey, websiteURL, websiteKey string) (string, error) {
	return solveCaptcha(capSolverBaseURL, apiKey, map[string]any{
		"type":       "ReCaptchaV2TaskProxyLess",
		"websiteURL": websiteURL,
		"websiteKey": websiteKey,
	}, "gRecaptchaResponse", true)
}

func capSolverSolveRecaptchaV3(apiKey, websiteURL, websiteKey, pageAction string) (string, error) {
	task := map[string]any{
		"type":       "ReCaptchaV3TaskProxyLess",
		"websiteURL": websiteURL,
		"websiteKey": websiteKey,
	}
	if pageAction != "" {
		task["pageAction"] = pageAction
	}
	return solveCaptcha(capSolverBaseURL, apiKey, task, "gRecaptchaResponse", true)
}

func capSolverSolveTurnstile(apiKey, websiteURL, websiteKey, cData string) (string, error) {
	task := map[string]any{
		"type":       "AntiTurnstileTaskProxyLess",
		"websiteURL": websiteURL,
		"websiteKey": websiteKey,
	}
	if cData != "" {
		task["metadata"] = map[string]any{"cdata": cData}
	}
	return solveCaptcha(capSolverBaseURL, apiKey, task, "token", true)
}
