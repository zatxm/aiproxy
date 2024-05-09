package types

type GeminiCompletionRequest struct {
	GeminiCompletionResponse
}

type GeminiCompletionResponse struct {
	Type  string `json:"type"`
	Index string `json:"index"`
}

// 流式响应模型对话，省略部分不常用参数
type StreamGenerateContent struct {
	// 自定义的model
	Model string `json:"model,omitempty"`
	// 必需,当前与模型对话的内容
	// 对于单轮查询此值为单个实例
	// 对于多轮查询，此字段为重复字段，包含对话记录和最新请求
	Contents []*GeminiContent `json:"contents"`
	// 可选,开发者集系统说明,目前仅支持文字
	SystemInstruction []*GeminiContent `json:"systemInstruction,omitempty"`
	// 可选,用于模型生成和输出的配置选项
	GenerationConfig *GenerationConfig `json:"generationConfig,omitempty"`
}

type GeminiContent struct {
	Parts []*GeminiPart `json:"parts"`
	Role  string        `json:"role"`
}

// Union field data can be only one of the following
type GeminiPart struct {
	// 文本
	Text string `json:"text,omitempty"`
	// 原始媒体字节
	MimeType string `json:"mimeType,omitempty"` //image/png等
	Data     string `json:"data,omitempty"`     //媒体格式的原始字节,使用base64编码的字符串
	// 基于URI的数据
	UriMimeType string `json:"mimeType,omitempty"` //可选,源数据的IANA标准MIME类型
	FileUri     string `json:"fileUri,omitempty"`  //必需,URI值
}

type GenerationConfig struct {
	// 将停止生成输出的字符序列集(最多5个)
	// 如果指定，API将在第一次出现停止序列时停止
	// 该停止序列不会包含在响应中
	StopSequences []string `json:"stopSequences,omitempty"`
	// 生成的候选文本的输出响应MIME类型
	// 支持的mimetype：text/plain(默认)文本输出,application/json JSON响应
	ResponseMimeType string `json:"responseMimeType,omitempty"`
	// 要返回的已生成响应数
	// 目前此值只能设置为1或者未设置默认为1
	CandidateCount int `json:"candidateCount,omitempty"`
	// 候选内容中包含的词元数量上限
	// 默认值因模型而异，请参阅getModel函数返回的Model的Model.output_token_limit属性
	MaxOutputTokens int `json:"maxOutputTokens,omitempty"`
	// 控制输出的随机性
	// 默认值因模型而异，请参阅getModel函数返回的Model的Model.temperature属性
	// 值的范围为[0.0, 2.0]
	Temperature float64 `json:"temperature,omitempty"`
	// 采样时要考虑的词元的最大累积概率
	// 该模型使用Top-k和核采样的组合
	// 词元根据其分配的概率进行排序，因此只考虑可能性最大的词元
	// Top-k采样会直接限制要考虑的最大词元数量，而Nucleus采样则会根据累计概率限制词元数量
	// 默认值因模型而异，请参阅getModel函数返回的Model的Model.top_p属性
	TopP float64 `json:"topP,omitempty"`
	// 采样时要考虑的词元数量上限
	// 模型使用核采样或合并Top-k和核采样,Top-k采样考虑topK集合中概率最高的词元
	// 通过核采样运行的模型不允许TopK设置
	// 默认值因模型而异,请参阅getModel函数返回的Model的Model.top_k属性
	// Model中的topK字段为空表示模型未应用Top-k采样,不允许对请求设置topK
	TopK int `json:"topK,omitempty"`
}
