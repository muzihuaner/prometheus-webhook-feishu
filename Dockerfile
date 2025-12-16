# 使用官方 Python 镜像
FROM python:3.8-slim

# 设置工作目录
WORKDIR /app

# 复制依赖文件并安装
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt -i https://mirrors.tuna.tsinghua.edu.cn/pypi/web/simple

# 复制所有项目文件
COPY . .

# 暴露端口
EXPOSE 5000

# 运行应用
CMD ["python", "fs.py"]
