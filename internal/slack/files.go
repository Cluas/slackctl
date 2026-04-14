package slack

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"strconv"
)

// FileInfo represents detailed Slack file metadata.
type FileInfo struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Title              string   `json:"title,omitempty"`
	Filetype           string   `json:"filetype,omitempty"`
	Mimetype           string   `json:"mimetype,omitempty"`
	Size               int      `json:"size,omitempty"`
	User               string   `json:"user,omitempty"`
	Created            int64    `json:"created,omitempty"`
	URLPrivate         string   `json:"url_private,omitempty"`
	URLPrivateDownload string   `json:"url_private_download,omitempty"`
	Permalink          string   `json:"permalink,omitempty"`
	Channels           []string `json:"channels,omitempty"`
}

// FileListResult holds files.list results.
type FileListResult struct {
	Files []FileInfo `json:"files"`
	Total int        `json:"total"`
}

func parseFileInfo(m map[string]any) FileInfo {
	fi := FileInfo{
		ID:                 stringVal(m, "id"),
		Name:               stringVal(m, "name"),
		Title:              stringVal(m, "title"),
		Filetype:           stringVal(m, "filetype"),
		Mimetype:           stringVal(m, "mimetype"),
		Size:               intVal(m, "size"),
		User:               stringVal(m, "user"),
		URLPrivate:         stringVal(m, "url_private"),
		URLPrivateDownload: stringVal(m, "url_private_download"),
		Permalink:          stringVal(m, "permalink"),
	}
	if v, ok := m["created"].(float64); ok {
		fi.Created = int64(v)
	}
	if channels, ok := m["channels"].([]any); ok {
		for _, ch := range channels {
			if s, ok := ch.(string); ok {
				fi.Channels = append(fi.Channels, s)
			}
		}
	}
	return fi
}

// GetFileInfo fetches detailed file metadata via files.info.
func (c *Client) GetFileInfo(fileID string) (*FileInfo, error) {
	resp, err := c.API("files.info", map[string]string{
		"file": fileID,
	})
	if err != nil {
		return nil, err
	}
	fileData := toRecord(resp["file"])
	fi := parseFileInfo(fileData)
	return &fi, nil
}

// DeleteFile deletes a file via files.delete.
func (c *Client) DeleteFile(fileID string) error {
	_, err := c.API("files.delete", map[string]string{
		"file": fileID,
	})
	return err
}

// ListFiles lists files via files.list with optional filters.
func (c *Client) ListFiles(channelID, userID, types string, limit int) (*FileListResult, error) {
	params := map[string]string{
		"count": strconv.Itoa(min(limit, 100)),
	}
	if channelID != "" {
		params["channel"] = channelID
	}
	if userID != "" {
		params["user"] = userID
	}
	if types != "" {
		params["types"] = types
	}

	var allFiles []FileInfo
	total := 0
	for {
		resp, err := c.API("files.list", params)
		if err != nil {
			return nil, err
		}
		files := getArray(resp, "files")
		for _, f := range files {
			allFiles = append(allFiles, parseFileInfo(toRecord(f)))
		}
		if paging, ok := resp["paging"].(map[string]any); ok {
			if t, ok := paging["total"].(float64); ok {
				total = int(t)
			}
			page := intVal(paging, "page")
			pages := intVal(paging, "pages")
			if page >= pages || len(allFiles) >= limit {
				break
			}
			params["page"] = strconv.Itoa(page + 1)
		} else {
			break
		}
	}
	if len(allFiles) > limit {
		allFiles = allFiles[:limit]
	}
	return &FileListResult{Files: allFiles, Total: total}, nil
}

// UploadFileV2 uploads a file using the V2 three-step flow:
// 1. files.getUploadURLExternal — get upload URL + file ID
// 2. POST file content to the upload URL
// 3. files.completeUploadExternal — associate file with channel
func (c *Client) UploadFileV2(filename string, content io.Reader, contentLength int64, channelID, threadTS, title, initialComment string) (*FileInfo, error) {
	// Step 1: Get upload URL
	params := map[string]string{
		"filename": filename,
		"length":   strconv.FormatInt(contentLength, 10),
	}
	resp, err := c.API("files.getUploadURLExternal", params)
	if err != nil {
		return nil, fmt.Errorf("getUploadURLExternal: %w", err)
	}
	uploadURL, _ := resp["upload_url"].(string)
	fileID, _ := resp["file_id"].(string)
	if uploadURL == "" || fileID == "" {
		return nil, fmt.Errorf("getUploadURLExternal: missing upload_url or file_id")
	}

	// Step 2: Upload file content to the upload URL
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("upload multipart: %w", err)
	}
	if _, err := io.Copy(part, content); err != nil {
		return nil, fmt.Errorf("upload copy: %w", err)
	}
	writer.Close()

	uploadResp, err := c.RawRequest("POST", uploadURL, &body, writer.FormDataContentType())
	if err != nil {
		return nil, fmt.Errorf("upload POST: %w", err)
	}
	defer uploadResp.Body.Close()
	if uploadResp.StatusCode < 200 || uploadResp.StatusCode >= 300 {
		return nil, fmt.Errorf("upload POST: HTTP %d", uploadResp.StatusCode)
	}

	// Step 3: Complete upload — associate file with channel
	filesParam := []map[string]string{{"id": fileID}}
	if title != "" {
		filesParam[0]["title"] = title
	}
	completeParams := map[string]string{
		"files":      JSONString(filesParam),
		"channel_id": channelID,
	}
	if threadTS != "" {
		completeParams["thread_ts"] = threadTS
	}
	if initialComment != "" {
		completeParams["initial_comment"] = initialComment
	}
	_, err = c.API("files.completeUploadExternal", completeParams)
	if err != nil {
		return nil, fmt.Errorf("completeUploadExternal: %w", err)
	}

	// Fetch file info for the response
	fi, err := c.GetFileInfo(fileID)
	if err != nil {
		// Upload succeeded but info fetch failed — return minimal info
		return &FileInfo{ID: fileID, Name: filename}, nil
	}
	return fi, nil
}

// DownloadFile downloads a file from url to dest using authenticated request.
func (c *Client) DownloadFile(fileURL string, dest io.Writer) error {
	resp, err := c.RawRequest("GET", fileURL, nil, "")
	if err != nil {
		return fmt.Errorf("download request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download: HTTP %d", resp.StatusCode)
	}
	if _, err := io.Copy(dest, resp.Body); err != nil {
		return fmt.Errorf("download write: %w", err)
	}
	return nil
}
