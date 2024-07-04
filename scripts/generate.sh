#!/usr/bin/env bash
# 生成一个dockmon的项目
# $1 为新项目名，如"project-name"，如果不传此参数则项目名为"dockmon"
# $2 为版本号，如"v1.0.0"，如果不传次参数则默认从master获取最新代码

projectName="dockmon"
projectVersion="main"

if [ "$1" != "" ]; then
  projectName="$1"
fi

if [ "$2" != "" ]; then
  projectVersion="$2"
fi

projectDir=$(pwd)/$projectName

rm -rf "$projectDir"

git clone -b "$projectVersion" https://dockmon.git "$projectDir"

rm -rf "$projectDir"/.git

grep -rl 'dockmon' "$projectDir" | xargs sed -i "" "s/github.com\/seakee\/dockmon/$projectName/g"

grep -rl 'dockmon' "$projectDir" | xargs sed -i "" "s/dockmon/$projectName/g"

echo "Success！"
