package har

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	http "github.com/bogdanfinn/fhttp"
	"github.com/zatxm/any-proxy/internal/config"
	"github.com/zatxm/any-proxy/pkg/jscrypt"
	"github.com/zatxm/fhblade"
)

var (
	bxTemp      = `[{"key":"api_type","value":"js"},{"key":"p","value":1},{"key":"f","value":"5658246a2142c8eb528707b5e9dd130e"},{"key":"n","value":"%s"},{"key":"wh","value":"ebcb832548b5ab8087d8a5a43f8d236c|72627afbfd19a741c7da1732218301ac"},{"key":"enhanced_fp","value":[{"key":"webgl_extensions","value":"ANGLE_instanced_arrays;EXT_blend_minmax;EXT_color_buffer_half_float;EXT_disjoint_timer_query;EXT_float_blend;EXT_frag_depth;EXT_shader_texture_lod;EXT_texture_compression_bptc;EXT_texture_compression_rgtc;EXT_texture_filter_anisotropic;EXT_sRGB;KHR_parallel_shader_compile;OES_element_index_uint;OES_fbo_render_mipmap;OES_standard_derivatives;OES_texture_float;OES_texture_float_linear;OES_texture_half_float;OES_texture_half_float_linear;OES_vertex_array_object;WEBGL_color_buffer_float;WEBGL_compressed_texture_s3tc;WEBGL_compressed_texture_s3tc_srgb;WEBGL_debug_renderer_info;WEBGL_debug_shaders;WEBGL_depth_texture;WEBGL_draw_buffers;WEBGL_lose_context;WEBGL_multi_draw"},{"key":"webgl_extensions_hash","value":"58a5a04a5bef1a78fa88d5c5098bd237"},{"key":"webgl_renderer","value":"WebKit WebGL"},{"key":"webgl_vendor","value":"WebKit"},{"key":"webgl_version","value":"WebGL 1.0 (OpenGL ES 2.0 Chromium)"},{"key":"webgl_shading_language_version","value":"WebGL GLSL ES 1.0 (OpenGL ES GLSL ES 1.0 Chromium)"},{"key":"webgl_aliased_line_width_range","value":"[1, 10]"},{"key":"webgl_aliased_point_size_range","value":"[1, 2047]"},{"key":"webgl_antialiasing","value":"yes"},{"key":"webgl_bits","value":"8,8,24,8,8,0"},{"key":"webgl_max_params","value":"16,64,32768,1024,32768,32,32768,31,16,32,1024"},{"key":"webgl_max_viewport_dims","value":"[32768, 32768]"},{"key":"webgl_unmasked_vendor","value":"Google Inc. (NVIDIA Corporation)"},{"key":"webgl_unmasked_renderer","value":"ANGLE (NVIDIA Corporation, NVIDIA GeForce RTX 3060 Ti/PCIe/SSE2, OpenGL 4.5.0)"},{"key":"webgl_vsf_params","value":"23,127,127,10,15,15,10,15,15"},{"key":"webgl_vsi_params","value":"0,31,30,0,31,30,0,31,30"},{"key":"webgl_fsf_params","value":"23,127,127,10,15,15,10,15,15"},{"key":"webgl_fsi_params","value":"0,31,30,0,31,30,0,31,30"},{"key":"webgl_hash_webgl","value":"7b9cf50d61f3da6ba617d04d2a217c48"},{"key":"user_agent_data_brands","value":"Not.A/Brand,Chromium,Google Chrome"},{"key":"user_agent_data_mobile","value":false},{"key":"navigator_connection_downlink","value":10},{"key":"navigator_connection_downlink_max","value":null},{"key":"network_info_rtt","value":150},{"key":"network_info_save_data","value":false},{"key":"network_info_rtt_type","value":null},{"key":"screen_pixel_depth","value":24},{"key":"navigator_device_memory","value":8},{"key":"navigator_languages","value":"zh-CN,en"},{"key":"window_inner_width","value":0},{"key":"window_inner_height","value":0},{"key":"window_outer_width","value":1920},{"key":"window_outer_height","value":1057},{"key":"browser_detection_firefox","value":false},{"key":"browser_detection_brave","value":false},{"key":"audio_codecs","value":"{\"ogg\":\"probably\",\"mp3\":\"probably\",\"wav\":\"probably\",\"m4a\":\"maybe\",\"aac\":\"probably\"}"},{"key":"video_codecs","value":"{\"ogg\":\"probably\",\"h264\":\"probably\",\"webm\":\"probably\",\"mpeg4v\":\"\",\"mpeg4a\":\"\",\"theora\":\"\"}"},{"key":"media_query_dark_mode","value":true},{"key":"headless_browser_phantom","value":false},{"key":"headless_browser_selenium","value":false},{"key":"headless_browser_nightmare_js","value":false},{"key":"document__referrer","value":""},{"key":"window__ancestor_origins","value":"%s"},{"key":"window__tree_index","value":[2]},{"key":"window__tree_structure","value":"[[],[],[]]"},{"key":"window__location_href","value":"https://tcr9i.chat.openai.com/v2/2.3.0/enforcement.0087e749a89110af598a5fae60fc4762.html#%s"},{"key":"client_config__sitedata_location_href","value":"%s"},{"key":"client_config__surl","value":"%s"},{"key":"mobile_sdk__is_sdk"},{"key":"client_config__language","value":null},{"key":"navigator_battery_charging","value":true},{"key":"audio_fingerprint","value":"124.04347527516074"}]},{"key":"fe","value":"[\"DNT:1\",\"L:zh-CN\",\"D:24\",\"PR:1\",\"S:1920,1080\",\"AS:1920,1080\",\"TO:-480\",\"SS:true\",\"LS:true\",\"IDB:true\",\"B:false\",\"ODB:true\",\"CPUC:unknown\",\"PK:Linux x86_64\",\"CFP:1941002709\",\"FR:false\",\"FOS:false\",\"FB:false\",\"JSF:Arial,Courier,Courier New,Helvetica,Times,Times New Roman\",\"P:Chrome PDF Viewer,Chromium PDF Viewer,Microsoft Edge PDF Viewer,PDF Viewer,WebKit built-in PDF\",\"T:0,false,false\",\"H:16\",\"SWF:false\"]"},{"key":"ife_hash","value":"411c34615014b9640db1d4c3f65dbee5"},{"key":"cs","value":1},{"key":"jsbd","value":"{\"HL\":5,\"NCE\":true,\"DT\":\"\",\"NWD\":\"false\",\"DOTO\":1,\"DMTO\":1}"}]`
	arkoseDatas = map[string]*mom{
		// auth
		"0A1D34FC-659D-4E23-B17B-694DCFCF6A6C": &mom{
			WindowAncestorOrigins:            `["https://auth0.openai.com"]`,
			ClientConfigSitedataLocationHref: "https://auth0.openai.com/u/login/password",
			ClientConfigSurl:                 "https://tcr9i.chat.openai.com",
			SiteUrl:                          "https://auth0.openai.com",
		},
		// chat3
		"3D86FBBA-9D22-402A-B512-3420086BA6CC": &mom{
			WindowAncestorOrigins:            `["https://chat.openai.com"]`,
			ClientConfigSitedataLocationHref: "https://chat.openai.com/",
			ClientConfigSurl:                 "https://tcr9i.chat.openai.com",
			SiteUrl:                          "https://chat.openai.com",
		},
		// chat4
		"35536E1E-65B4-4D96-9D97-6ADB7EFF8147": &mom{
			WindowAncestorOrigins:            `["https://chat.openai.com"]`,
			ClientConfigSitedataLocationHref: "https://chat.openai.com/",
			ClientConfigSurl:                 "https://tcr9i.chat.openai.com",
			SiteUrl:                          "https://chat.openai.com",
		},
		// platform
		"23AAD243-4799-4A9E-B01D-1166C5DE02DF": &mom{
			WindowAncestorOrigins:            `["https://platform.openai.com"]`,
			ClientConfigSitedataLocationHref: "https://platform.openai.com/account/api-keys",
			ClientConfigSurl:                 "https://openai-api.arkoselabs.com",
			SiteUrl:                          "https://platform.openai.com",
		},
	}
)

type mom struct {
	WindowAncestorOrigins            string
	WindowLocationHref               string
	ClientConfigSitedataLocationHref string
	ClientConfigSurl                 string
	SiteUrl                          string
	Hars                             []*harData
}

type KvPair struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type harData struct {
	Url     string
	Method  string
	Bv      string
	Bx      string
	Headers http.Header
	Body    url.Values
}

func (h *harData) Clone() *harData {
	b, _ := fhblade.Json.Marshal(h)
	var cloned harData
	fhblade.Json.Unmarshal(b, &cloned)
	return &cloned
}

type harFileData struct {
	Log harLogData `json:"log"`
}

type harLogData struct {
	Entries []harEntrie `json:"entries"`
}

type harEntrie struct {
	Request         harRequest `json:"request"`
	StartedDateTime string     `json:"startedDateTime"`
}

type harRequest struct {
	Method   string         `json:"method"`
	URL      string         `json:"url"`
	Headers  []KvPair       `json:"headers,omitempty"`
	PostData harRequestBody `json:"postData,omitempty"`
}

type harRequestBody struct {
	Params []KvPair `json:"params"`
}

func GenerateBx(key string, bt int64) string {
	m := arkoseDatas[key]
	return fmt.Sprintf(bxTemp,
		jscrypt.GenerateN(bt),
		m.WindowAncestorOrigins,
		m.WindowLocationHref,
		m.ClientConfigSitedataLocationHref,
		m.ClientConfigSurl)
}

func GetArkoseDatas() map[string]*mom {
	return arkoseDatas
}

func Parse() error {
	var harPath []string
	harDirPath := config.V().HarsPath
	err := filepath.Walk(harDirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// 判断是否为普通文件（非文件夹）
		if !info.IsDir() {
			ext := filepath.Ext(info.Name())
			if ext == ".har" {
				harPath = append(harPath, path)
			}
		}
		return nil
	})
	if err != nil {
		return errors.New("Error: please put HAR files in harPool directory!")
	}
	if len(harPath) > 0 {
		for pk := range harPath {
			file, err := os.ReadFile(harPath[pk])
			if err != nil {
				fmt.Println(err)
				continue
			}
			var hfData harFileData
			err = fhblade.Json.Unmarshal(file, &hfData)
			if err != nil {
				fmt.Println(err)
				continue
			}
			data := &harData{}
			tagKey := "openai.com/fc/gt2/"
			arkoseKey := "abc"
			for k := range hfData.Log.Entries {
				v := hfData.Log.Entries[k]
				arkoseKey = "abc"
				if !strings.Contains(v.Request.URL, tagKey) || v.StartedDateTime == "" {
					continue
				}
				data.Url = v.Request.URL
				data.Method = v.Request.Method
				data.Headers = make(http.Header)
				for hk := range v.Request.Headers {
					h := v.Request.Headers[hk]
					if strings.HasPrefix(h.Name, ":") || h.Name == "content-length" || h.Name == "connection" || h.Name == "cookie" {
						continue
					}
					if h.Name == "user-agent" {
						data.Bv = h.Value
					} else {
						data.Headers.Set(h.Name, h.Value)
					}
				}
				if data.Bv == "" {
					continue
				}
				data.Body = make(url.Values)
				for pk := range v.Request.PostData.Params {
					p := v.Request.PostData.Params[pk]
					if p.Name == "bda" {
						pcipher, _ := url.QueryUnescape(p.Value)
						t, _ := time.Parse(time.RFC3339, v.StartedDateTime)
						bt := t.Unix()
						bw := jscrypt.GenerateBw(bt)
						bx, err := jscrypt.Decrypt(pcipher, data.Bv+bw)
						if err != nil {
							fmt.Println(err)
						} else {
							data.Bx = bx
						}
					} else if p.Name != "rnd" {
						q, _ := url.QueryUnescape(p.Value)
						data.Body.Set(p.Name, q)
						if p.Name == "public_key" {
							if _, ok := arkoseDatas[p.Value]; ok {
								arkoseKey = p.Value
							}
						}
					}
				}
				if data.Bx != "" {
					break
				}
			}
			if data.Bx != "" && arkoseKey != "abc" {
				arkoseDatas[arkoseKey].Hars = append(arkoseDatas[arkoseKey].Hars, data)
			}
		}
		return err
	}
	return errors.New("Empty HAR files in harPool directory!")
}
