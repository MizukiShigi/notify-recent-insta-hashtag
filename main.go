package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

const (
	InstagramAPIHost        = "https://graph.facebook.com"
	LINEAPIHost             = "https://api.line.me"
	InstagramHashtagIDURL   = "/v20.0/ig_hashtag_search"
	InstagramRecentMediaURL = "/v20.0/:tag_id/recent_media"
	LINEBroadcastURL        = "/v2/bot/message/broadcast"
	MaxLINEMessageCount     = 5
)

type HashtagSearchResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

type RecentMediaResponse struct {
	Data []struct {
		Permalink string `json:"permalink"`
	} `json:"data"`
}

type LINEBroadcastRequest struct {
	Messages []LINEBroadcastMessage `json:"messages"`
}

type LINEBroadcastMessage struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf(".envファイルを読み込めませんでした: %v", err)
	}

	instaUserID := os.Getenv("INSTAGRAM_USER_ID")
	if instaUserID == "" {
		log.Fatal("INSTAGRAM_USER_IDが設定されていません")
	}

	instaAccessToken := os.Getenv("INSTAGRAM_ACCESS_TOKEN")
	if instaAccessToken == "" {
		log.Fatal("INSTAGRAM_ACCESS_TOKENが設定されていません")
	}

	instaTagNameList := os.Getenv("INSTAGRAM_TAG_NAME_LIST")
	if instaTagNameList == "" {
		log.Fatal("INSTAGRAM_TAG_NAME_LISTが設定されていません")
	}

	lineAccessToken := os.Getenv("LINE_CHANNEL_ACCESS_TOKEN")
	if lineAccessToken == "" {
		log.Fatal("LINE_CHANNEL_ACCESS_TOKENが設定されていません")
	}

	tagNameListSlice := strings.Split(instaTagNameList, ",")

	hashtagID := getHashtagID(tagNameListSlice[0], instaUserID, instaAccessToken)

	tagIDList := hashtagID.Data

	for _, tag := range tagIDList {
		recentMediaResponse := getRecentMediaUrlList(tag.ID, instaUserID, instaAccessToken)
		messages := make([]string, len(recentMediaResponse.Data))
		for i, permalink := range recentMediaResponse.Data {
			messages[i] = permalink.Permalink
		}
		broadcastMessage(messages, lineAccessToken)
	}
}

func getHashtagID(tagName string, userID string, accessToken string) HashtagSearchResponse {
	tagBaseURL := InstagramAPIHost + InstagramHashtagIDURL
	params := url.Values{}
	params.Add("user_id", userID)
	params.Add("q", tagName)
	params.Add("access_token", accessToken)
	tagUrl := tagBaseURL + "?" + params.Encode()

	tagRes, err := http.Get(tagUrl)
	if err != nil {
		log.Fatalf("ハッシュタグ検索リクエストに失敗しました: %v", err)
	}

	body, err := io.ReadAll(tagRes.Body)
	if err != nil {
		log.Fatalf("レスポンスの読み込みに失敗しました: %v", err)
	}

	var hashtagSearchResponse HashtagSearchResponse
	err = json.Unmarshal(body, &hashtagSearchResponse)
	if err != nil {
		log.Fatalf("ハッシュタグ検索レスポンスのパースに失敗しました: %v", err)
	}

	if len(hashtagSearchResponse.Data) == 0 {
		log.Fatal("ハッシュタグが見つかりませんでした")
	}

	return hashtagSearchResponse
}

func getRecentMediaUrlList(tagID string, userID string, accessToken string) RecentMediaResponse {
	recentMediaBaseURL := InstagramAPIHost + InstagramRecentMediaURL
	recentMediaBaseURL = strings.Replace(recentMediaBaseURL, ":tag_id", tagID, 1)
	params := url.Values{}
	params.Add("user_id", userID)
	params.Add("fields", "permalink")
	params.Add("access_token", accessToken)
	params.Add("limit", "50")
	recentMediaURL := recentMediaBaseURL + "?" + params.Encode()

	res, err := http.Get(recentMediaURL)
	if err != nil {
		log.Fatalf("最新メディアリクエストに失敗しました: %v", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Fatalf("レスポンスの読み込みに失敗しました: %v", err)
	}

	var recentMediaResponse RecentMediaResponse
	err = json.Unmarshal(body, &recentMediaResponse)
	if err != nil {
		log.Fatalf("最新メディアレスポンスのパースに失敗しました: %v", err)
	}

	return recentMediaResponse
}

func broadcastMessage(messages []string, accessToken string) {
	messageBroadcastURL := LINEAPIHost + LINEBroadcastURL
	for i := 0; i < len(messages); i += MaxLINEMessageCount {
		end := i + MaxLINEMessageCount
		if end > len(messages) {
			end = len(messages)
		}

		messageBatch := messages[i:end]
		LINEBroadcastMessageList := []LINEBroadcastMessage{}
		for _, message := range messageBatch {
			LINEBroadcastMessageList = append(
				LINEBroadcastMessageList,
				LINEBroadcastMessage{Type: "text", Text: message},
			)
		}

		broadcastRequest := LINEBroadcastRequest{Messages: LINEBroadcastMessageList}
		broadcastRequestJSON, err := json.Marshal(broadcastRequest)
		if err != nil {
			log.Fatalf("LINEブロードキャストリクエストのJSONエンコードに失敗しました: %v", err)
		}

		req, err := http.NewRequest("POST", messageBroadcastURL, bytes.NewBuffer(broadcastRequestJSON))
		if err != nil {
			log.Fatalf("LINEブロードキャストリクエストの作成に失敗しました: %v", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+accessToken)

		client := &http.Client{}
		res, err := client.Do(req)
		if err != nil {
			log.Fatalf("LINEブロードキャストリクエスト時にエラーが発生しました: %v", err)
		}

		if res.StatusCode != http.StatusOK {
			log.Fatalf("LINEブロードキャストリクエストに失敗しました: %v", res.Status)
		}
	}
}
