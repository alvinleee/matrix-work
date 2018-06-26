# 疑惑

- 验证者的leader在向各个follower收取交易进行合并去重的时候，本地的交易池是不是要加锁，暂停从钱包获取交易？

- 无效交易的的信息的数据结构是什么样的？

- 区块结构中交易存的是一个交易指针的切片，那把区块加入区块链时，如何获取交易内容？

- pandoc --template=pm-template.latex --pdf-engine=xelatex question.md -o question.pdf