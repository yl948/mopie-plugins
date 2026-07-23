# Mopie 插件仓库

这是 Mopie 的官方插件仓库，提供各种扩展功能。

## 如何使用

1. 在 Mopie 中打开 **设置 → 插件管理**
2. 切换到 **插件市场** 标签页
3. 选择要安装的插件，点击 **安装** 按钮

## 如何开发插件

### 插件目录结构

```
your_plugin/
├── manifest.json     # 插件元数据
├── README.md         # 插件说明文档
└── ...               # 其他文件
```

### manifest.json 格式

```json
{
  "name": "your_plugin",
  "title": "你的插件名称",
  "author": "作者名",
  "description": "插件描述",
  "version": "1.0",
  "logoUrl": "",
  "githubUrl": "https://github.com/your/repo",
  "helpDocUrl": "https://github.com/your/repo/wiki",
  "payImageUrl": "",
  "configField": [
    {
      "fieldName": "api_key",
      "fieldType": "String",
      "label": "API Key",
      "helperText": "请输入你的API Key",
      "defaultValue": "",
      "required": true
    }
  ],
  "dependencies": {
    "appVersion": ">=1.0.0"
  }
}
```

### 提交插件

1. Fork 本仓库
2. 在 `plugins/` 目录下创建你的插件文件夹
3. 编写 `manifest.json` 和相关文件
4. 更新 `registry.json` 添加你的插件信息
5. 创建 Pull Request

### 配置字段类型

- `String`: 文本输入
- `Bool`: 开关选择
- `Enum`: 下拉选择（需要提供 `enumValues`）
- `Number`: 数字输入

## 插件列表

| 插件名称 | 描述 | 版本 | 作者 |
|---------|------|------|------|
| 过滤器及排序规则配置下载 | 提供过滤器及排序规则配置 | 1.0 | NaNaKo_ |

## 联系我们

如有问题或建议，请在 GitHub Issues 中反馈。
