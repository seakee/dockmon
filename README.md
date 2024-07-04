# Dockmon

## 项目介绍

Dockmon 是一个用 Go 编写的 Docker容器监视器，可以从 Docker 容器中收集日志并存储到 MySQL 数据库中。它支持解析结构化和非结构化日志，监控 Docker 事件以动态收集新启动容器的日志。

## 功能特性

- 收集指定容器的日志
- 支持结构化和非结构化日志解析
- 监控 Docker 事件，动态收集新启动容器的日志
- 日志存储到 MySQL 数据库，包含容器 ID 和名称

## 环境要求

- Go 1.22 或更高版本
- Docker
- MySQL
- Redis

## 安装与使用

### 本地运行

1. 克隆仓库

    ```sh
    git clone https://github.com/seakee/dockmon.git
    cd dockmon
    ```

2. 修改配置文件
   ```sh
   cp bin/configs/local.json.example bin/configs/local.json
   ```
   修改 `bin/configs/local.json` 文件redis、mysql 等配置.`system.jwt_secret`参数不能为空，建议至少 32 位以上随机字符。

3. 初始化数据库

   将 `bin/data/sql` 目录下的 sql 文件导入到 MySQL 数据库
 
4. 编译项目

    ```sh
    make build
    ```

5. 运行程序

    ```sh
    make run
    ```

### Docker 运行

#### 构建 Docker 镜像

1. 构建 Docker 镜像

    ```sh
    make docker-build
    ```

#### 运行 Docker 容器
1. 运行 Docker 容器

    ```sh
    make docker-run 
    ```

   或者手动运行 Docker 容器并指定环境变量：

    ```sh
    docker run -d --name $(PROJECT_NAME) \
		-p 8085:8080 \
		-it \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v $(CONFIG_DIR):/bin/configs \
		-v /bin/docker:/bin/docker \
		-e APP_NAME=$(PROJECT_NAME) \
		$(IMAGE_NAME)
   ```

## 许可证
本项目采用 MIT 许可证。

