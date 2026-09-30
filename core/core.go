package zrokcore
import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
)
var apiEndpoint = "https://api.zrok.io"
func SetApiEndpoint(ep string) {
	if ep != "" {
		apiEndpoint = ep
	}
}
func jok(data any) string {
	b, _ := json.Marshal(map[string]any{"ok": true, "data": data})
	return string(b)
}
func jerr(err error) string {
	b, _ := json.Marshal(map[string]any{"ok": false, "error": err.Error()})
	return string(b)
}
func zreq(method, path, token string, body any) ([]byte, error) {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, apiEndpoint+"/api/v1/"+path, rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-TOKEN", token)
	if body != nil {
		req.Header.Set("Content-Type", "application/zrok.v1+json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, errors.New("HTTP " + strconv.Itoa(resp.StatusCode) + ": " + string(b))
	}
	return b, nil
}
func Overview(token string) string {
	b, err := zreq("GET", "overview", token, nil)
	if err != nil {
		return jerr(err)
	}
	var v any
	json.Unmarshal(b, &v)
	return jok(v)
}
func CreateShare(token, envZId, target, uniqueName string) string {
	body := map[string]any{
		"envZId": envZId, "shareMode": "public",
		"frontendSelection": []string{"public"},
		"backendMode":       "proxy", "backendProxyEndpoint": target,
		"authScheme": "none", "authUsers": []any{}, "reserved": true,
	}
	if uniqueName != "" {
		body["uniqueName"] = uniqueName
	}
	b, err := zreq("POST", "share", token, body)
	if err != nil {
		return jerr(err)
	}
	var v any
	json.Unmarshal(b, &v)
	return jok(v)
}
func DeleteShare(token, envZId, shareToken string) string {
	_, err := zreq("DELETE", "unshare", token, map[string]any{
		"envZId": envZId, "shareToken": shareToken, "reserved": true,
	})
	if err != nil {
		return jerr(err)
	}
	return jok(nil)
}
