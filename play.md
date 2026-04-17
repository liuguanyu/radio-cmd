# Radio.cn 播放逻辑分析文档

## 1. 实际发现的API和数据结构

通过Python嗅探脚本分析，radio.cn的电台数据和播放地址直接通过API获取：

### API端点
- **电台列表API**: `https://ytmsout.radio.cn/web/appBroadcast/list?categoryId=0&provinceCode=0`
- **分类列表API**: `https://ytmsout.radio.cn/web/appCategory/list/all`
- **省份列表API**: `https://ytmsout.radio.cn/web/appProvince/list/all`

### API返回格式
```json
{
  "code": 0,
  "message": "SUCCESS",
  "data": [
    {
      "contentId": "639",
      "title": "中国之声",
      "subtitle": "正在直播：全国新闻联播",
      "image": "https://ytmedia.radio.cn/...",
      "playUrlLow": "http://ytlive.radio.cn/130/radios/10639/index_10639.m3u8?type=1&key=...",
      "mp3PlayUrlLow": "http://ytcast2.radio.cn/110/radios/30639/index_30639.m3u8?type=1&key=...",
      "mp3PlayUrlHigh": "http://ytcast2.radio.cn/110/radios/40639/index_40639.m3u8?type=1&key=...",
      "playUrlMulti": "http://ytcast2.radio.cn/110/radios/40639/index_40639.m3u8?type=1&key=..."
    }
  ]
}
```

## 2. 已获取的电台列表（部分）

共获取到 **19个电台**，包括：

1. 中国之声 (ID: 639)
2. 经济之声 (ID: 640)
3. 环球资讯广播 (ID: 692)
4. 音乐之声 (ID: 641)
5. 中国交通广播 (ID: 653)
6. 文艺之声 (ID: 648)
7. 大湾区之声 (ID: 645)
8. 经典音乐广播 (ID: 642)
9. 台海之声 (ID: 643)
10. 神州之声 (ID: 644)
11. 香港之声 (ID: 646)
12. 民族之声 (ID: 647)
13. 老年之声 (ID: 649)
14. 藏语广播 (ID: 650)
15. 维吾尔语广播 (ID: 651)
16. 中国乡村之声 (ID: 654)
17. 哈萨克语广播 (ID: 655)
18. 南海之声 (ID: 664)
19. 英语资讯广播 CGTN Radio (ID: 734)

## 3. 播放URL格式

每个电台提供4个播放URL选项：

1. **playUrlLow**: 低质量HLS流（推荐用于弱网环境）
   - 格式: `http://ytlive.radio.cn/130/radios/{id}/index_{id}.m3u8?type=1&key=...`

2. **mp3PlayUrlLow**: 低质量MP3流
   - 格式: `http://ytcast2.radio.cn/110/radios/{id}/index_{id}.m3u8?type=1&key=...`

3. **mp3PlayUrlHigh**: 高质量MP3流（推荐）
   - 格式: `http://ytcast2.radio.cn/110/radios/{id}/index_{id}.m3u8?type=1&key=...`

4. **playUrlMulti**: 多码率流
   - 格式: `http://ytcast2.radio.cn/110/radios/{id}/index_{id}.m3u8?type=1&key=...`

所有URL都包含：
- `type=1` 参数
- `key=...` 认证参数（动态生成）
- `time=...` 时间戳参数

## 4. Go应用程序实现方案

### 项目结构
```
radio-cmd/
├── cmd/
│   └── radio-cmd/
│       └── main.go          # 主程序入口
├── pkg/
│   ├── radio/
│   │   ├── station.go       # 电台数据结构
│   │   ├── client.go        # API客户端
│   │   └── player.go        # 播放器控制
│   └── tui/
│       ├── app.go           # TUI主应用
│       ├── styles.go        # Lipgloss样式定义
│       └── components/      # UI组件
│           ├── station_list.go
│           ├── player_controls.go
│           └── status_bar.go
├── internal/
│   └── config/
│       └── config.go        # 配置管理
├── go.mod
├── go.sum
└── README.md
```

### 技术栈
- **TUI框架**: `github.com/charmbracelet/bubbletea`
- **样式美化**: `github.com/charmbracelet/lipgloss`
- **HLS播放**: 使用系统播放器或Go音频库
- **HTTP客户端**: `net/http` + `encoding/json`

### 实现步骤

1. **初始化Go模块**
   - 创建go.mod
   - 安装依赖

2. **数据模型**
   ```go
   type Station struct {
       ContentID      string `json:"contentId"`
       Title          string `json:"title"`
       Subtitle       string `json:"subtitle"`
       Image          string `json:"image"`
       PlayUrlLow     string `json:"playUrlLow"`
       Mp3PlayUrlLow  string `json:"mp3PlayUrlLow"`
       Mp3PlayUrlHigh string `json:"mp3PlayUrlHigh"`
       PlayUrlMulti   string `json:"playUrlMulti"`
   }
   ```

3. **API客户端**
   - 实现GetStations()方法获取电台列表
   - 缓存机制减少API调用

4. **播放功能**
   - 调用系统播放器（如mpv, vlc, afplay）
   - 或使用Go音频库播放HLS流

5. **TUI界面**
   - 电台列表显示
   - 键盘导航（↑/↓/Enter/q）
   - 播放状态显示
   - 当前播放电台高亮

### 用户交互设计

```
╔═══════════════════════════════════════════════╗
║              📻 Radio.cn CLI Player            ║
╠═══════════════════════════════════════════════╣
║                                               ║
║  > 中国之声          [正在播放]               ║
║    经济之声                                    ║
║    音乐之声                                    ║
║    文艺之声                                    ║
║    经典音乐广播                                ║
║                                               ║
╠═══════════════════════════════════════════════╣
║  Status: 播放中 | 正在直播：全国新闻联播      ║
║  [Enter]播放  [Space]暂停  [Q]退出           ║
╚═══════════════════════════════════════════════╝
```

### 键盘快捷键
- `↑` / `k`: 向上选择
- `↓` / `j`: 向下选择
- `Enter`: 播放选中的电台
- `Space`: 暂停/继续
- `q` / `Ctrl+C`: 退出程序
- `r`: 刷新电台列表
- `?`: 显示帮助

## 5. 注意事项

1. **认证参数有效期**
   - URL中的key和time参数有时效性
   - 需要定期刷新（建议每小时重新获取）

2. **网络错误处理**
   - 实现重试机制
   - 提供离线模式（使用缓存的电台列表）

3. **跨平台播放器**
   - macOS: `afplay` 或 `mpv`
   - Linux: `mpv` 或 `vlc`
   - Windows: `vlc` 或 `ffplay`

4. **性能优化**
   - 电台列表本地缓存
   - 异步加载电台图标（可选）
   - 播放状态管理

## 6. 下一步行动

✅ 已完成：运行嗅探脚本获取电台列表和播放API
⏭️ 下一步：初始化Go模块和项目结构

## 4. 播放技术实现

- 播放使用 HTML5 Audio 或 HLS.js 播放器
- 使用 WebSocket 或轮询维持连接状态
- 音频流通常有较短的 TTL，可能需要定期刷新

## 5. Go 应用程序实现建议

1. **预加载电台列表**：
   - 在应用程序启动时，从静态文件加载电台列表
   - 或者首次运行时通过 API 获取并缓存

2. **播放功能**：
   - 使用 `github.com/hajimehoshi/ebiten` 或 `github.com/asticode/go-astisub` 处理音频流
   - 使用 `https://github.com/muesli/termbox-go` 处理 TUI
   - 选择 `github.com/charmbracelet/bubbletea` 作为 TUI 框架
   - 使用 `github.com/charmbracelet/lipgloss` 进行样式美化

3. **播放逻辑**：
   - 用户选择电台 → 发送请求到 `https://www.radio.cn/api/radio/play`
   - 解析响应获取播放 URL → 使用 Go 音频库播放
   - 实现播放状态管理（播放/暂停/停止）

## 6. 注意事项

- 电台播放 URL 可能需要 Cookie 或认证头
- 服务器可能有速率限制
- 播放 URL 可能有有效期，需要定期刷新
- 需要处理网络错误和播放失败的情况
- 建议实现播放历史和缓存机制

## 7. 下一步行动

1. 运行 `radio_sniffer_advanced.py` 获取实际数据
2. 创建 Go 项目结构
3. 实现电台列表加载功能
4. 实现 TUI 界面
5. 实现播放功能
6. 添加错误处理和用户体验优化

## 8. 预期输出示例

```bash
$ radio-cmd

[音乐之声]   (1001)   [播放中]
[中国之声]   (1002)
[经济之声]   (1003)
[文艺之声]   (1004)
[都市之声]   (1005)

使用 ↑/↓ 选择，Enter 播放，Q 退出
```

> 注意：实际 API 端点和 URL 格式需要通过运行嗅探脚本验证。以上为基于典型广播网站模式的分析。