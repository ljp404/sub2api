//go:build unit

package admin

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestParameterPricingPreviewUsesSettlementCalculator(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := NewChannelHandler(nil, nil, nil)
	router.POST("/preview", h.PreviewParameterPricing)
	for _, test := range []struct {
		dimensions   string
		code         int
		amount       string
		errorMessage string
	}{
		{`{"quality":"high"}`, 200, "0.08432960", ""},
		{`{"quality":"unknown"}`, 400, "", "当前计费模式为：图片（参数），参数 quality（图片质量）的值 \"unknown\" 不受支持，可选值：high"},
		{`{}`, 400, "", "当前计费模式为：图片（参数），缺少必填参数 quality（图片质量），可选值：high"},
	} {
		body := `{"rule":{"version":1,"unit":"image","base_price":"0.052706","factors":[{"key":"quality","values":{"high":"1"}}]},"input":{"dimensions":` +
			test.dimensions + `,"quantity":2,"multiplier":"0.8"}}`
		req := httptest.NewRequest("POST", "/preview", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, test.code, rec.Code)
		if test.code == 200 {
			require.Equal(t, test.amount, gjson.Get(rec.Body.String(), "data.actual_cost").String())
		} else {
			require.Equal(t, test.errorMessage, gjson.Get(rec.Body.String(), "message").String())
		}
	}
}

func TestParameterPricingChannelAPIPreservesRule(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/", strings.NewReader(`{"models":["gpt-image-2"],"platform":"openai","billing_mode":"parameter","parameter_pricing":{"version":1,"unit":"image","base_price":"0.052706","factors":[]}}`))
	c.Request.Header.Set("Content-Type", "application/json")
	var request channelModelPricingRequest
	require.NoError(t, c.ShouldBindJSON(&request))
	pricing := pricingRequestToService([]channelModelPricingRequest{request}, true)
	require.Equal(t, "0.052706", pricing[0].ParameterPricing.BasePrice)
	require.Equal(t, request.ParameterPricing, pricingToResponse(&pricing[0]).ParameterPricing)
}
