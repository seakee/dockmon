#!/usr/bin/env bash
# 进入项目根目录
cd /share/Container/WorkSpace/dockmon
# 移除之前的执行日志
rm -rf deploy.log
# 构建镜像版本号
version=$(date "+%Y%m%d%H%M")
# 上一个版本容器ID
lastContainerId=$(docker ps -a -q --filter "name=dockmon" | awk '{print $1}')
# 停止上一个版本容器
docker stop $lastContainerId >> deploy.log 2>&1
# 删除上一个版本容器
docker rm $lastContainerId >> deploy.log 2>&1
# 删除上一个版本镜像
docker rmi -f $(docker images --filter "reference=dockmon" -q) >> deploy.log 2>&1
#构建镜像
docker build -t "dockmon:"$version . >> deploy.log 2>&1
# 删除所有无名的镜像(悬空镜像)
docker rmi -f $(docker images -f "dangling=true" -q) >> deploy.log 2>&1
#部署最新版本
docker run -d --name dockmon \
           -e RUN_ENV=prod \
           -v /share/Container/data/dockmon/configs:/bin/configs \
           -v /share/Container/data/dockmon/logs:/bin/logs \
           -p 8085:8080 --restart always "dockmon:"$version >> deploy.log 2>&1