package sarin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"go.aykhans.me/sarin/internal/types"
)

const (
	captchaPollInterval = 1 * time.Second
	captchaPollTimeout  = 120 * time.Second
)

var captchaHTTPClient = &http.Client{Timeout: 5 * time.Second}

// solveCaptcha creates a task on the given captcha service and polls until it is solved,
// returning the extracted token from the solution object.
//
// baseURL is the service API base (e.g. "https://api.2captcha.com").
// task is the task payload the service expects (type + service-specific fields).
// solutionKey is the field name in the solution object that holds the token.
// taskIDIsString controls whether taskId is sent back as a string (CapSolver UUIDs)
// or a JSON number (2Captcha, Anti-Captcha).
//
// It can return the following errors:
//   - types.ErrCaptchaKeyEmpty
//   - types.CaptchaRequestError
//   - types.CaptchaAPIError
//   - types.CaptchaPollTimeoutError
//   - types.CaptchaSolutionKeyError
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

// captchaCreateTask submits a task to the captcha service and returns the assigned taskId.
// The taskId is normalized to a string: numeric IDs are preserved via json.RawMessage,
// and quoted string IDs (CapSolver UUIDs) have their surrounding quotes stripped.
//
// It can return the following errors:
//   - types.CaptchaRequestError
//   - types.CaptchaAPIError
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
	taskID := strings.Trim(string(result.TaskID), `"`)
	if taskID == "" {
		return "", types.NewCaptchaAPIError("createTask", "EMPTY_TASK_ID", "service returned a successful response with no taskId")
	}
	return taskID, nil
}

// captchaPollResult polls the getTaskResult endpoint at captchaPollInterval until the task
// is solved, an error is returned by the service, or the overall captchaPollTimeout is hit.
//
// It can return the following errors:
//   - types.CaptchaPollTimeoutError
//   - types.CaptchaRequestError
//   - types.CaptchaAPIError
//   - types.CaptchaSolutionKeyError
func captchaPollResult(baseURL, apiKey, taskID, solutionKey string, taskIDIsString bool) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), captchaPollTimeout)
	defer cancel()

	ticker := time.NewTicker(captchaPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", types.NewCaptchaPollTimeoutError(taskID)
		case <-ticker.C:
			token, err := captchaGetTaskResult(baseURL, apiKey, taskID, solutionKey, taskIDIsString)
			if errors.Is(err, types.ErrCaptchaProcessing) {
				continue
			}
			if err != nil {
				return "", err
			}
			return token, nil
		}
	}
}

// captchaGetTaskResult fetches a single task result from the captcha service.
//
// It can return the following errors:
//   - types.ErrCaptchaProcessing
//   - types.CaptchaRequestError
//   - types.CaptchaAPIError
//   - types.CaptchaSolutionKeyError
func captchaGetTaskResult(baseURL, apiKey, taskID, solutionKey string, taskIDIsString bool) (string, error) {
	var bodyMap map[string]any
	if taskIDIsString {
		bodyMap = map[string]any{"clientKey": apiKey, "taskId": taskID}
	} else {
		bodyMap = map[string]any{"clientKey": apiKey, "taskId": json.Number(taskID)}
	}

	data, err := json.Marshal(bodyMap)
	if err != nil {
		return "", types.NewCaptchaRequestError("getTaskResult", err)
	}

	resp, err := captchaHTTPClient.Post(
		baseURL+"/getTaskResult",
		"application/json",
		bytes.NewReader(data),
	)
	if err != nil {
		return "", types.NewCaptchaRequestError("getTaskResult", err)
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
		return "", types.NewCaptchaRequestError("getTaskResult", err)
	}

	if result.ErrorID != 0 {
		return "", types.NewCaptchaAPIError("getTaskResult", result.ErrorCode, result.ErrorDescription)
	}

	if result.Status == "processing" || result.Status == "idle" {
		return "", types.ErrCaptchaProcessing
	}

	token, ok := result.Solution[solutionKey]
	if !ok {
		return "", types.NewCaptchaSolutionKeyError(solutionKey)
	}
	tokenStr, ok := token.(string)
	if !ok {
		return "", types.NewCaptchaSolutionKeyError(solutionKey)
	}

	return tokenStr, nil
}

// ======================================== 2Captcha ========================================

const twoCaptchaBaseURL = "https://api.2captcha.com"

// twoCaptchaSolveRecaptchaV2 solves a Google reCAPTCHA v2 challenge via 2Captcha.
//
// It can return the following errors:
//   - types.ErrCaptchaKeyEmpty
//   - types.CaptchaRequestError
//   - types.CaptchaAPIError
//   - types.CaptchaPollTimeoutError
//   - types.CaptchaSolutionKeyError
func twoCaptchaSolveRecaptchaV2(apiKey, websiteURL, websiteKey string) (string, error) {
	return solveCaptcha(twoCaptchaBaseURL, apiKey, map[string]any{
		"type":       "RecaptchaV2TaskProxyless",
		"websiteURL": websiteURL,
		"websiteKey": websiteKey,
	}, "gRecaptchaResponse", false)
}

// twoCaptchaSolveRecaptchaV3 solves a Google reCAPTCHA v3 challenge via 2Captcha.
// pageAction may be empty.
//
// It can return the following errors:
//   - types.ErrCaptchaKeyEmpty
//   - types.CaptchaRequestError
//   - types.CaptchaAPIError
//   - types.CaptchaPollTimeoutError
//   - types.CaptchaSolutionKeyError
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

// twoCaptchaSolveTurnstile solves a Cloudflare Turnstile challenge via 2Captcha.
// cData may be empty.
//
// It can return the following errors:
//   - types.ErrCaptchaKeyEmpty
//   - types.CaptchaRequestError
//   - types.CaptchaAPIError
//   - types.CaptchaPollTimeoutError
//   - types.CaptchaSolutionKeyError
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

// antiCaptchaSolveRecaptchaV2 solves a Google reCAPTCHA v2 challenge via Anti-Captcha.
//
// It can return the following errors:
//   - types.ErrCaptchaKeyEmpty
//   - types.CaptchaRequestError
//   - types.CaptchaAPIError
//   - types.CaptchaPollTimeoutError
//   - types.CaptchaSolutionKeyError
func antiCaptchaSolveRecaptchaV2(apiKey, websiteURL, websiteKey string) (string, error) {
	return solveCaptcha(antiCaptchaBaseURL, apiKey, map[string]any{
		"type":       "RecaptchaV2TaskProxyless",
		"websiteURL": websiteURL,
		"websiteKey": websiteKey,
	}, "gRecaptchaResponse", false)
}

// antiCaptchaSolveRecaptchaV3 solves a Google reCAPTCHA v3 challenge via Anti-Captcha.
// pageAction may be empty. minScore is hardcoded to 0.3 (the loosest threshold) because
// Anti-Captcha rejects the request without it.
//
// It can return the following errors:
//   - types.ErrCaptchaKeyEmpty
//   - types.CaptchaRequestError
//   - types.CaptchaAPIError
//   - types.CaptchaPollTimeoutError
//   - types.CaptchaSolutionKeyError
func antiCaptchaSolveRecaptchaV3(apiKey, websiteURL, websiteKey, pageAction string) (string, error) {
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

// antiCaptchaSolveHCaptcha solves an hCaptcha challenge via Anti-Captcha.
// Anti-Captcha returns hCaptcha tokens under "gRecaptchaResponse" (not "token").
//
// It can return the following errors:
//   - types.ErrCaptchaKeyEmpty
//   - types.CaptchaRequestError
//   - types.CaptchaAPIError
//   - types.CaptchaPollTimeoutError
//   - types.CaptchaSolutionKeyError
func antiCaptchaSolveHCaptcha(apiKey, websiteURL, websiteKey string) (string, error) {
	return solveCaptcha(antiCaptchaBaseURL, apiKey, map[string]any{
		"type":       "HCaptchaTaskProxyless",
		"websiteURL": websiteURL,
		"websiteKey": websiteKey,
	}, "gRecaptchaResponse", false)
}

// antiCaptchaSolveTurnstile solves a Cloudflare Turnstile challenge via Anti-Captcha.
// cData may be empty.
//
// It can return the following errors:
//   - types.ErrCaptchaKeyEmpty
//   - types.CaptchaRequestError
//   - types.CaptchaAPIError
//   - types.CaptchaPollTimeoutError
//   - types.CaptchaSolutionKeyError
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

// capSolverSolveRecaptchaV2 solves a Google reCAPTCHA v2 challenge via CapSolver.
//
// It can return the following errors:
//   - types.ErrCaptchaKeyEmpty
//   - types.CaptchaRequestError
//   - types.CaptchaAPIError
//   - types.CaptchaPollTimeoutError
//   - types.CaptchaSolutionKeyError
func capSolverSolveRecaptchaV2(apiKey, websiteURL, websiteKey string) (string, error) {
	return solveCaptcha(capSolverBaseURL, apiKey, map[string]any{
		"type":       "ReCaptchaV2TaskProxyLess",
		"websiteURL": websiteURL,
		"websiteKey": websiteKey,
	}, "gRecaptchaResponse", true)
}

// capSolverSolveRecaptchaV3 solves a Google reCAPTCHA v3 challenge via CapSolver.
// pageAction may be empty.
//
// It can return the following errors:
//   - types.ErrCaptchaKeyEmpty
//   - types.CaptchaRequestError
//   - types.CaptchaAPIError
//   - types.CaptchaPollTimeoutError
//   - types.CaptchaSolutionKeyError
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

// capSolverSolveTurnstile solves a Cloudflare Turnstile challenge via CapSolver.
// cData may be empty. CapSolver nests cData under a "metadata" object.
//
// It can return the following errors:
//   - types.ErrCaptchaKeyEmpty
//   - types.CaptchaRequestError
//   - types.CaptchaAPIError
//   - types.CaptchaPollTimeoutError
//   - types.CaptchaSolutionKeyError
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
