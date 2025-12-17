# Kubernetes APIサーバーとの間の接続を一時的に止める方法

## 通信の遮断
```
sudo iptables -A OUTPUT -d 127.0.0.1 -p tcp --dport 6443 -j REJECT
```

## 通信の復旧
```
sudo iptables -D OUTPUT -d 127.0.0.1 -p tcp --dport 6443 -j REJECT
```