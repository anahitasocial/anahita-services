package handlers

import (
	"anahita-libs/data/entities"
	"anahita-libs/middleware"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

// getViewer returns the viewer from the context
func getViewer(c echo.Context) *entities.Viewer {
	return middleware.GetViewer(c)
}

func getClientIP(c echo.Context) string {
	ip := c.Request().Header.Get("X-Forwarded-For")
	if ip != "" {
		return strings.TrimSpace(strings.Split(ip, ",")[0])
	}

	ip = c.Request().Header.Get("X-Real-IP")
	if ip != "" {
		return ip
	}

	return echo.ExtractIPDirect()(c.Request())
}

// Looking up IP address location using the ipapi.co service
func getIPLocation(ip string) IPLocation {
	var (
		err        error
		ipLocation = IPLocation{}
		client     = http.Client{}
	)

	if strings.HasPrefix(ip, "172.") ||
		strings.HasPrefix(ip, "192.168.") ||
		strings.HasPrefix(ip, "10.") ||
		ip == "127.0.0.1" ||
		ip == "::1" ||
		ip == "" {
		return IPLocation{
			City:    "localhost",
			Country: "development",
		}
	}

	url := fmt.Sprintf("https://ipapi.co/%s/json/", ip)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Println(err)
	}

	req.Header.Set("User-Agent", "ipapi.co/#go-v1.5")
	resp, err := client.Do(req)

	if err != nil {
		log.Println(err)
	}

	defer resp.Body.Close()

	json_map := make(map[string]interface{})
	err = json.NewDecoder(resp.Body).Decode(&json_map)

	if err != nil {
		return ipLocation
	}

	if json_map["error"] == true {
		errorMsg := json_map["reason"].(string)
		log.Println(errorMsg)
		return ipLocation
	}

	ipLocation.City = json_map["city"].(string)
	ipLocation.Country = json_map["country_name"].(string)

	return ipLocation
}
