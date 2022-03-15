package main

import (
	"encoding/json"
	"fmt"
	"github.com/childe/gohangout/value_render"
	"github.com/golang/glog"
	"gopkg.in/resty.v1"
	"math/rand"
	"strings"
	"sync"
	"time"
)

type ZincOutput struct {
	config map[interface{}]interface{}

	// 索引名
	index value_render.ValueRender

	// 写入并发度
	concurrency int

	// 存储批量请求
	batchRequests     []string
	batchRequestsChan chan []string

	// 每次批量请求数
	batchSize int

	// 批量请求定时刷新时间，单位s
	batchFlushInterval int

	// zinc地址，支持多个节点
	addresses []string

	// zinc用户名
	username string

	// zinc密码
	password string

	client *resty.Client

	waitGroup sync.WaitGroup
	lock      sync.Mutex
}

const (
	messageTemplate = `
{ "index" : { "_index" : "%s" } }
%s
`
	DefaultBatchSize          = 100
	DefaultBatchFlushInterval = 10
	DefaultBatchConcurrency   = 4
)

// New gohangout plugin规范，函数名必须为New
func New(config map[interface{}]interface{}) interface{} {
	output := &ZincOutput{config: config}
	if addresses, ok := config["addresses"]; ok {
		for _, v := range addresses.([]interface{}) {
			output.addresses = append(output.addresses, v.(string))
		}
	} else {
		glog.Fatal("addresses must be set in zinc output")
	}
	if username, ok := config["username"]; ok {
		output.username = username.(string)
	} else {
		glog.Fatal("output config illegal, not specify username")
	}

	if password, ok := config["password"]; ok {
		output.password = password.(string)
	} else {
		glog.Fatal("output config illegal, not specify password")
	}

	if index, ok := config["index"]; ok {
		output.index = value_render.GetValueRender(index.(string))
	} else {
		glog.Fatal("output config illegal, not specify index pattern")
	}

	var (
		concurrency, batchSize, batchFlushInterval int
	)

	if v, ok := config["batch_size"]; ok {
		batchSize = v.(int)
		if batchSize <= 0 {
			glog.Fatal("output config illegal, batch size must greater than 0")
		}
	} else {
		batchSize = DefaultBatchSize
	}
	output.batchSize = batchSize

	if v, ok := config["batch_flush_interval"]; ok {
		batchFlushInterval = v.(int)
	} else {
		batchFlushInterval = DefaultBatchFlushInterval
	}
	output.batchFlushInterval = batchFlushInterval

	if v, ok := config["concurrency"]; ok {
		concurrency = v.(int)
	} else {
		concurrency = DefaultBatchConcurrency
	}
	output.concurrency = concurrency

	output.batchRequests = make([]string, 0)
	output.batchRequestsChan = make(chan []string, 3*concurrency)
	output.client = resty.New()

	// 启动定时刷新Ticker，防止流量过低时，写入延迟太大的问题
	ticker := time.NewTicker(time.Second * time.Duration(output.batchFlushInterval))
	go func() {
		for range ticker.C {
			if len(output.batchRequests) > 0 {
				output.lock.Lock()
				output.batchRequestsChan <- output.batchRequests
				output.batchRequests = make([]string, 0)
				output.lock.Unlock()
			}
		}
	}()

	output.waitGroup = sync.WaitGroup{}
	output.waitGroup.Add(concurrency)
	// 根据并发度，启动N个goroutine消费批量请求
	for i := 0; i < output.concurrency; i++ {
		go func() {
			defer output.waitGroup.Done()
			for {
				select {
				case requests := <-output.batchRequestsChan:
					// 收到空请求，表示进程结束，直接退出
					if len(requests) == 0 {
						return
					}
					output.processRequests(requests)
				}
			}
		}()
	}

	glog.Infof("zinc output config, address: %v, index: %v, username: %v, password: %v, batch_size: %v, "+
		"batch_flush_interval: %v, concurrency: %v", output.addresses, output.index, output.username, output.password,
		output.batchSize, output.batchFlushInterval, output.concurrency)
	return output
}

func (z *ZincOutput) Emit(event map[string]interface{}) {
	index := z.index.Render(event).(string)
	marshal, err := json.Marshal(event)
	if err != nil {
		glog.Errorf("marshal message failed: %v", err)
		return
	}
	z.lock.Lock()
	z.batchRequests = append(z.batchRequests, fmt.Sprintf(messageTemplate, index, string(marshal)))
	if len(z.batchRequests) >= z.batchSize {
		z.batchRequestsChan <- z.batchRequests
		z.batchRequests = make([]string, 0)
	}
	z.lock.Unlock()
}

func (z *ZincOutput) processRequests(requests []string) {
	_, err := z.client.R().
		SetBasicAuth(z.username, z.password).
		SetBody(strings.Join(requests, "\n")).
		Post(fmt.Sprintf("%s/api/_bulk", strings.TrimRight(z.selectAddress(), "/")))
	if err != nil {
		glog.Errorf("write messages to zinc failed: %v", err)
	} else {
		glog.Infof("write messages to zinc success, size: %v", len(requests))
	}
}

// 从多个地址中选择一个，此处仅简单随机获取
func (z *ZincOutput) selectAddress() string {
	return z.addresses[rand.Intn(len(z.addresses))]
}

func (z *ZincOutput) Shutdown() {
	glog.Infof("zinc output is shutting down...")
	// 发送空请求，结束goroutine
	for i := 0; i < z.concurrency; i++ {
		z.batchRequestsChan <- make([]string, 0)
	}
	z.waitGroup.Wait()
	close(z.batchRequestsChan)
	glog.Infof("zinc output shutdown completed")
}
