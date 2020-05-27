# Golang Client for Monitor
## 安装
```
gb vendor fetch github.com/finogeeks/dendrite/skunkworks/monitor/go-client
# or
go get github.com/finogeeks/dendrite/skunkworks/monitor/go-client
```
## 启动参数（环境变量）

变量|说明|默认值
---|---|---
ENABLE_MONITOR|是否打开监控|false
MONITOR_PORT|exporter端口|9092
MONITOR_PATH|exporter路径|/metrics

## 获取监控数据
访问：```http://<SERVER_URL>:<MONITOR_PORT>/MONITOR_PATH```.
比如，本地调试时可以直接在浏览器访问```http://127.0.0.1:9092/metrics```查看已添加的监控指标。

## 接口说明
[Interface](https://github.com/finogeeks/dendrite/skunkworks/monitor/go-client/src/master/monitor/interface.go)
### 指标名和标签名
* 指标名和标签名作为指标的唯一签名，唯一确定了一个指标，比如: ```mesons_synapse_requests{code, method}```
* 调用NewCounter等接口创建新的指标时，整个程序内对同一个指标签名只能创建一次，否则进程会强制退出；但指标的取值可以不同，比如: ```mesons_synapse_requests{code=200, method=GET}```, ```mesons_synapse_requests{code=404, method=POST}```
* 对于一些需要在多个package中创建的相同签名的指标，可以将该指标定义在公共的package中，比如：

```golang
package metrics

var Counter monitor.LabeledCounter = monitor.NewLabeledCounter("mesons_synapse_requests", []string{"code", "method"})
```
```golang
package main

import "metrics"

func main() {
    metrics.Counter.WithLabelValues("404", "POST").Inc()
    metrics.Counter.With(monitor.Labels{"code": "200", "method": "GET"}).Add(5)
}
```
* 指标的命名必须能够唯一区分该指标，推荐使用```<namespace>_<subsystem>_<metric>```的格式
* 更多指标和标签命名的最佳实践请参考[METRIC AND LABEL NAMING](https://prometheus.io/docs/practices/naming/)

## 例子
[Code Demo](https://github.com/finogeeks/dendrite/skunkworks/monitor/go-client/src/master/test/test_main.go)

### Output
```
demo_test_counter 10
demo_test_labeled_counter{code="200",method="GET"} 5
demo_test_labeled_counter{code="404",method="POST"} 1

demo_test_gauge 90
demo_test_labeled_gauge{code="200",method="GET"} 1.4972397686894338e+09
demo_test_labeled_gauge{code="404",method="POST"} 200

demo_test_histogram_bucket{le="0.005"} 0
demo_test_histogram_bucket{le="0.01"} 0
demo_test_histogram_bucket{le="0.025"} 0
demo_test_histogram_bucket{le="0.05"} 0
demo_test_histogram_bucket{le="0.1"} 0
demo_test_histogram_bucket{le="0.25"} 0
demo_test_histogram_bucket{le="0.5"} 0
demo_test_histogram_bucket{le="1"} 0
demo_test_histogram_bucket{le="2.5"} 0
demo_test_histogram_bucket{le="5"} 0
demo_test_histogram_bucket{le="10"} 0
demo_test_histogram_bucket{le="+Inf"} 1
demo_test_histogram_sum 8888
demo_test_histogram_count 1
demo_test_labeled_histogram_bucket{code="200",method="GET",le="8000"} 1
demo_test_labeled_histogram_bucket{code="200",method="GET",le="+Inf"} 1
demo_test_labeled_histogram_sum{code="200",method="GET"} 7777
demo_test_labeled_histogram_count{code="200",method="GET"} 1
demo_test_labeled_histogram_bucket{code="404",method="POST",le="8000"} 0
demo_test_labeled_histogram_bucket{code="404",method="POST",le="+Inf"} 1
demo_test_labeled_histogram_sum{code="404",method="POST"} 9999
demo_test_labeled_histogram_count{code="404",method="POST"} 1

demo_test_labeled_summary{code="200",method="GET",quantile="0.9"} 5000
demo_test_labeled_summary{code="200",method="GET",quantile="0.99"} 5000
demo_test_labeled_summary{code="200",method="GET",quantile="0.999"} 5000
demo_test_labeled_summary_sum{code="200",method="GET"} 5000
demo_test_labeled_summary_count{code="200",method="GET"} 1
demo_test_labeled_summary{code="404",method="POST",quantile="0.9"} 2000
demo_test_labeled_summary{code="404",method="POST",quantile="0.99"} 2000
demo_test_labeled_summary{code="404",method="POST",quantile="0.999"} 2000
demo_test_labeled_summary_sum{code="404",method="POST"} 2000
demo_test_labeled_summary_count{code="404",method="POST"} 1
demo_test_summary{quantile="0.5"} 1000
demo_test_summary{quantile="0.9"} 1000
demo_test_summary{quantile="0.99"} 1000
demo_test_summary_sum 1000
demo_test_summary_count 1

demo_test_histogram_duration_seconds_bucket{le="0.005"} 0
demo_test_histogram_duration_seconds_bucket{le="0.01"} 0
demo_test_histogram_duration_seconds_bucket{le="0.025"} 0
demo_test_histogram_duration_seconds_bucket{le="0.05"} 0
demo_test_histogram_duration_seconds_bucket{le="0.1"} 0
demo_test_histogram_duration_seconds_bucket{le="0.25"} 0
demo_test_histogram_duration_seconds_bucket{le="0.5"} 0
demo_test_histogram_duration_seconds_bucket{le="1"} 0
demo_test_histogram_duration_seconds_bucket{le="2.5"} 1
demo_test_histogram_duration_seconds_bucket{le="5"} 1
demo_test_histogram_duration_seconds_bucket{le="10"} 1
demo_test_histogram_duration_seconds_bucket{le="+Inf"} 1
demo_test_histogram_duration_seconds_sum 1.004677512
demo_test_histogram_duration_seconds_count 1

demo_test_summary_duration_seconds{quantile="0.5"} 1.004695222
demo_test_summary_duration_seconds{quantile="0.9"} 1.004695222
demo_test_summary_duration_seconds{quantile="0.99"} 1.004695222
demo_test_summary_duration_seconds_sum 1.004695222
demo_test_summary_duration_seconds_count 1

demo_test_labeled_summary_duration_seconds{code="200",method="GET",quantile="0.5"} 0.203377195
demo_test_labeled_summary_duration_seconds{code="200",method="GET",quantile="0.9"} 0.203377195
demo_test_labeled_summary_duration_seconds{code="200",method="GET",quantile="0.99"} 0.203377195
demo_test_labeled_summary_duration_seconds_sum{code="200",method="GET"} 0.203377195
demo_test_labeled_summary_duration_seconds_count{code="200",method="GET"} 1
demo_test_labeled_summary_duration_seconds{code="404",method="POST",quantile="0.5"} 0.408277122
demo_test_labeled_summary_duration_seconds{code="404",method="POST",quantile="0.9"} 0.408277122
demo_test_labeled_summary_duration_seconds{code="404",method="POST",quantile="0.99"} 0.408277122
demo_test_labeled_summary_duration_seconds_sum{code="404",method="POST"} 0.408277122
demo_test_labeled_summary_duration_seconds_count{code="404",method="POST"} 1
```
