# 项目约定

## 提交推送规则

- 未经用户明确要求，禁止执行 git commit / push（代码改动完成也不得自行提交）
- 仅当用户明确要求提交或推送时，方可执行

## 忽略文件

以下文件禁止提交（应保持 git 忽略）：

- `.env` 及所有密钥文件（`*.key`、`*.pem`、证书、token 等）
- 运行数据与本地配置：`data/`、`compose/`、`logs/`、`*.db`、`*.sqlite3`
- 构建产物：`dist/`、`node_modules/`、编译二进制
- 系统/编辑器文件：`.DS_Store`、`.idea/`、`.vscode/`
