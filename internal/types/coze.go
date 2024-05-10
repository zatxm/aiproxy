package types

type CozeCompletionRequest struct {
	Conversation *CozeConversation `json:"conversation,omitempty"`
	// 作为上下文传递的聊天历史记录
	ChatHistory []*CozeApiChatMessage `json:"chat_history,omitempty"`
}

type CozeApiChatMessage struct {
	// 发送消息角色,user用户输入内容，assistant为Bot返回内容
	Role string `json:"role"`
	// 标识消息类型，主要用于区分role=assistant时Bot返回的消息
	Type string `json:"type"`
	// 消息内容
	Content string `json:"content"`
	// 消息内容类型,一般为text
	ContentType string `json:"content_type"`
}

type CozeConversation struct {
	Type string `json:"type,omitempty"`
	// 要进行会话聊天的Bot ID
	BotId string `json:"bot_id"`
	// 对话标识,需自己生成
	ConversationId string `json:"conversation_id,omitempty"`
	// 标识当前与Bot交互的用户，使用方自行维护此字段
	User string `json:"user"`
}

// Api请求
// https://www.coze.com/open/docs/chat?_lang=zh
type CozeApiChatRequest struct {
	// 要进行会话聊天的Bot ID
	BotId string `json:"bot_id"`
	// 对话标识,需自己生成
	ConversationId string `json:"conversation_id,omitempty"`
	// 标识当前与Bot交互的用户，使用方自行维护此字段
	User string `json:"user"`
	// 用户输入内容
	Query string `json:"query"`
	// 作为上下文传递的聊天历史记录
	ChatHistory []*CozeApiChatMessage `json:"chat_history,omitempty"`
	// 使用启用流式返回，目前本代码只处理true流式响应
	Stream bool `json:"stream"`
	// Bot中定义的变量信息，key是变量名，value是变量值
	CustomVariables map[string]interface{} `json:"custom_variables,omitempty"`
}

// Api返回,本代码只处理true流式响应
type CozeApiChatResponse struct {
	// 数据包事件，不同event会返回不同字段
	// message消息内容,done正常结束标志,error错误结束标志
	Event string `json:"event"`
	// event为error时返回的错误信息
	ErrorInformation map[string]interface{} `json:"error_information,omitempty"`
	// 增量返回的消息内容
	Message *CozeApiChatMessage `json:"message"`
	// 当前message是否结束,false未结束,true结束
	// 结束表示一条完整的消息已经发送完成,不代表整个模型返回结束
	IsFinish bool `json:"is_finish"`
	// 同一个index的增量返回属于同一条消息
	Index int `json:"index"`
	// 会话ID
	ConversationId string `json:"conversation_id"`
	// 序号
	SeqId int `json:"seq_id"`
	// 状态码,0表示调用成功
	Code int `json:"code,omitempty"`
	// 状态信息
	Msg string `json:"msg,omitempty"`
}
