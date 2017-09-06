# 简介
该命令用来创建指定编码方式的zip归档文件。七牛支持的[mkzip功能](http://developer.qiniu.com/docs/v6/api/reference/fop/mkzip.html)默认当前仅支持utf8编码方式，该编码方式打包的文件在Windows操作系统下面使用系统自带的unzip功能时，会造成中文文件名称乱码。该命令通过指定文件名称编码为gbk的方式可以解决这个问题。目前支持utf8（默认）和gbk（手动指定）两种编码方式。

**备注**：该命令只能对指定空间中的文件进行打包操作，支持的最大文件数量为1000。

# 命令
该命令名称为`mkzip`，对应的ufop实例名称为`ufop_prefix`+`mkzip`。

```
mkzip
/bucket/<UrlsafeBase64EncodedBucket>
/encoding/<UrlsafeBase64EncodedEncoding>
/url/<UrlsafeBase64EncodedURL>/alias/<UrlsafeBase64EncodedAlias>
/url/<UrlsafeBase64EncodedURL>/alias/<UrlsafeBase64EncodedAlias>
/ignore404/(0|1)
...
```

**PS: 参数有固定的顺序，可选参数可以不设置**

# 参数
|参数名|描述|可选|
|-------|---------|-----------|
|bucket|需要打包的文件所在的空间名称|必须|
|encoding|需要打包的文件名称的编码，支持gbk和utf8，默认为utf8|可选|
|url|需要打包的文件可访问的链接，必须存在于`bucket`中|至少指定一个链接|
|alias|需要打包的文件所对应的别名，和`url`配对使用|可以不设置|
|ignore404|如果指定的url中可能存在不在空间中的文件，可以指定该值为1，忽略404的文件|

**备注**：所有的的参数必须使用`UrlsafeBase64`编码方式编码。

# 配置
出于安全性的考虑，你可以根据实际的需求设置如下参数来控制mkzip功能的安全性：

|Key|Value|描述|
|--------|------------|----------------|
|mkzip_max_file_length|默认为100MB，单位：字节|允许打包的文件的单个文件最大字节长度|
|mkzip_max_file_count|默认为100个|允许打包的文件的最大总数量，最多支持1000|

如果需要自定义，你需要在`qufop.conf`的配置文件中添加这两项。

# 常见错误

|错误信息|描述|
|-------|------|
|invalid mkzip command format|发送的ufop的指令格式不正确，请参考上面的命令格式设置正确的指令|
|invalid mkzip paramter 'bucket'|指定的`bucket`参数不正确，必须是对原空间名称进行`urlsafe base64`编码后的值|
|invalid mkzip parameter 'encoding'|指定的`encoding`参数不正确，必须是对原编码名称进行`urlsafe base64`编码后的值|
|invalid mkzip parameter 'url'|指定的`url`列表中有一个不正确，必须是对资源链接进行`urlsafe base64`编码后的值|
|invalid mkzip parameter 'alias'|指定的`alias`列表中有一个不正确，必须是对文件别名进行`urlsafe base64`编码后的值|
|mkzip parameter 'url' format error|指定的`url`列表中有一个不正确，必须是正确的资源链接|
|invalid mkzip resource url|指定的`url`列表中有一个不正确，必须是正确的资源链接|
|duplicate mkzip resource alias|指定的`alias`列表中的别名有重复|
|zip file count exceeds the limit|需要压缩的文件数量超过了ufop的最大值限制，这个最大值在`mkzip.conf`里面设置|
|only support items less than 1000|需要压缩的文件数量超过了ufop的最大限制，目前代码最大允许1000个文件压缩|

# 示例

持久化的使用方式：

```
qn-mkzip
/bucket/aWYtcGJs
/encoding/Z2Jr
/url/aHR0cDovLzdwbjY0Yy5jb20xLnowLmdsYi5jbG91ZGRuLmNvbS8yMDE1LzAzLzIyL3Fpbml1Lm1wNA==/alias/5LiD54mb5a6j5Lyg54mH
/url/aHR0cDovLzdwbjY0Yy5jb20xLnowLmdsYi5jbG91ZGRuLmNvbS8yMDE1LzAzLzIyL3Fpbml1LnBuZw==
/url/aHR0cDovLzdwbjY0Yy5jb20xLnowLmdsYi5jbG91ZGRuLmNvbS8yMDE1LzAzLzI3LzEzLmpwZw==/alias/MjAxNS9waG90by5qcGc=
|saveas/aWYtcGJsOnFpbml1LnppcA==
```

上面的写法是格式化后便于理解，实际使用中没有换行符号。

其中`saveas`的参数为保存的目标空间和目标文件名的`Urlsafe Base64编码`。
