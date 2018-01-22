# unzip 服务

## 场景

有些客户是做在线教育的，在线教育的课程开发一般都是采用静态的页面技术，通过融合flash，mp4和html，js，css技术，把课程制作为可以直接打开的静态页面，然后将静态页面打包为一个zip文件，然后进行分发。这个场景的特点是这个压缩包里面可能存在少则几百，多则几千的小文件，如果是按每个文件上传到七牛的话，效率确实不高，不如将一个zip压缩包上传到七牛之后，在云端存储进行解压。

## 方案

七牛官方提供了各种图片处理和音视频处理服务。另外为了能够满足更加丰富的计算场景需求，七牛开放数据处理的平台接口，可以支持用户自行部署符合协议的计算程序到云端，就近从存储里面拖取文件进行计算，然后再将结果保存到存储中。这个技术，官方称之为[用户自定义计算服务](https://developer.qiniu.com/dora/manual/3687/ufop-directions-for-use)。

在本场景中，如果存在一个可以支持zip解压的用户自定义程序，那么可以直接从存储空间里面拖取zip文件，然后进行解压，再把解压后的文件传回存储中，这样就可以实现文件的云端解压功能。由于计算服务和存储服务同在一个机房中，可以有效提升整体的效率。

## 实现

用户自定义计算本质上是一个HTTP的服务，该客户遵循七牛定义的数据获取方式来从空间获取数据，然后将计算完成的结果或者直接输出到服务的HTTP回复中，然后通过七牛支持的saveas管道来将处理结果保存到空间，或者通过SDK自行将文件内容上传到空间，然后服务的HTTP的回复中只给出必要的元数据信息即可。

对于云端解压的场景，最适合的方式是将解压后的文件自行上传到存储，因为saveas管道只支持一个文件的保存。在将文件上传到存储之后，我们可以在服务的回复中给出解压后的文件列表供业务端使用。

## 服务

本项目中提供了基于Go语言的unzip服务的示范程序，该程序可以自行编译部署使用。

本程序的设计考虑到不同客户可能都会使用到这个服务，由于用户自定义计算（UFOP）应用的名称是全局唯一的，所以我们定义了一个 `ufop_prefix`的配置参数，用来标识不同的客户，当你需要部署该服务的使用，可以定义自己的 `ufop_prefix`，然后再注册UFOP服务的时候，就用 `ufop_prefix` + `unzip`作为 UFOP 应用的名字，这个名字也是后面程序调用的时候使用的名字。比如你定义 `ufop_prefix` 为 `qntest-` ，那么你使用 `qdoractl` 注册 UFOP 服务的时候，就是使用命令：

```
$ qdoractl register qntest-unzip -desc 'unzip files'
```

然后调用的时候也是使用这个名称来调用命令：

```
qntest-unzip/xxxx
```

这个 `ufop_prefix` 参数定义在配置文件 `qufop.conf` 中。

## 部署

在下载项目之后，可以直接使用项目下的 `UfopPlay/unzip/src/cross_build.sh` 来编译得到目标的二进制文件 `qufop` ，然后将其移动到部署目录 `UfopPlay/unzip/deploy/unzip`下面。

```
$ UfopPlay/unzip/deploy> 
.
├── Dockerfile
└── unzip
    ├── qufop
    ├── qufop.conf
    └── unzip.conf

1 directory, 4 files
```

其中 `unzip.conf` 是 unzip 服务相关的配置文件，内容如下：

```
{
	"access_key": "your access key",
	"secret_key": "your secret key",
	"unzip_max_zip_file_length":104857600,
	"unzip_max_file_length":104857600,
	"unzip_max_file_count":100
}
```

|参数|描述|
|----|----|
|access_key|账号的AK，因为我们需要把解压后的文件上传到空间，所以需要AK|
|secret_key|账号的SK，因为我们需要把解压后的文件上传到空间，所以需要SK|
|unzip_max_zip_file_length|待解压文件的最大大小，单位字节|
|unzip_max_file_length|压缩包文件中单个文件的最大大小，单位字节|
|unzip_max_file_count|压缩包文件中的总文件数量|

之所以会有后面的三个 `unzip_` 开头的三个配置选项，主要是出于安全考虑，因为有种攻击型压缩包文件可以释放出超级大的单个文件，耗尽计算资源，所以从互联网安全角度，我们加上几个限制，这几个参数根据自己实际的业务特点设置合理的数值即可。

在完成上面的准备工作之后，我们就可以打包 docker 镜像了，切换到 `Dockerfile` 文件所在的目录使用下面的命令打包镜像 ：

```
$ docker build . -t qntest-unzip:1.0
```

镜像打包完成之后，再使用下面命令推送镜像：

```
$ qdoractl push qntest-unzip:1.0
```

镜像文件推动到云端之后，可以直接登录到七牛后台，然后部署相关的运行实例就可以使用了。

## 用法

目前该服务支持的命令格式如下（实际调用的时候，请加上前缀）：

```
unzip/bucket/<encoded bucket>/prefix/<encoded prefix>/overwrite/<[0|1]>
```

|参数|描述|
|----|----|
|bucket|使用 UrlsafeBase64 编码方式编码的目标空间名称|
|prefix|使用 UrlsafeBase64 编码方式编码的目标文件前缀，可以不设置，默认为空，前缀主要用来模拟目录|
|overwrite|如果空间已有解压后的同名文件，是否覆盖上传，设置为1为覆盖，默认不覆盖|

该命令可以通过持久化数据处理的方式调用，或者在文件较小的时候使用实时数据处理的方式调用。具体请参考对应文档。