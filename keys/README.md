
# 生成离线 License

## 基本用法

```shell
# 生成 license（默认 999999 用户数限制）
go run -tags enterprise ./enterprise/cmd/coder generate-license -o /lyndon/iData/coder/keys/license.jwt -k /lyndon/iData/coder/keys/coder-publickey.pem -p /lyndon/iData/coder/keys/coder-privatekey.pem

./coder_2.27.1-devel+4b94c7fcc_linux_amd64 generate-license -o /tmp/coder/license.jwt -k /tmp/coder/coder-publickey.pem -p /tmp/coder/coder-privatekey.pem
```

## 指定用户数限制

```shell
# 生成 license，限制 100 用户
go run -tags enterprise ./enterprise/cmd/coder generate-license \
  --user-limit 100 \
  -o /lyndon/iData/coder/keys/license.jwt \
  -k /lyndon/iData/coder/keys/coder-publickey.pem \
  -p /lyndon/iData/coder/keys/coder-privatekey.pem

# 生成 license，限制 500 用户
./coder generate-license --user-limit 500 -o license.jwt
```

## 参数说明

| 参数            | 简写 | 环境变量                       | 说明         | 默认值   |
|-----------------|------|--------------------------------|------------|----------|
| `--output`      | -o   | CODER_LICENSE_OUTPUT           | 输出文件路径 | 标准输出 |
| `--public-key`  | -k   | CODER_LICENSE_PUBLIC_KEY_FILE  | 公钥输出路径 | -        |
| `--private-key` | -p   | CODER_LICENSE_PRIVATE_KEY_FILE | 私钥输出路径 | -        |
| `--user-limit`  | -    | CODER_LICENSE_USER_LIMIT       | 最大用户数   | 999999   |

## License 特性

- 有效期：30 年
- 包含所有企业版功能
- 用户数限制：可自定义（默认 999999）
