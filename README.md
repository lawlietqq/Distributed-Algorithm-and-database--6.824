![IMG_20230624_190427](https://github.com/lawlietqq/Distributed-Database-with-Raft/assets/92260319/b58c5677-6803-4c30-bfcf-addc104eed1c)# Distributed-Database-with-Raft
lab1 MapReduce模型
lab1 分布式概念比较好理解，就是一个简单的并行处理的概念，需要合理使用golang语言的map结构。

![image](https://github.com/lawlietqq/Distributed-Database-with-Raft/assets/92260319/3b2b75fc-d24f-48e2-aece-4559d5ed486c)

Lab2 Raft算法

Raft算法是所有四个lab的核心，也是lab3与lab4的理论基础
Raft将共识机制分解为三个小问题，分别是领导选举，日志复制以及安全性问题。
领导选举：
定义三种状态follower，candidate，leader，代码如下所示：

type State int

const (

    Follower State = iota // value --> 0
    
    Candidate             // value --> 1
    
    Leader                // value --> 2
    
)
raft的基本数据结构定义如下所示：

type Raft struct {

    mu        sync.Mutex          // Lock to protect shared access to this peer's state
    peers     []*labrpc.ClientEnd // RPC end points of all peers
    persister *Persister          // Object to hold this peer's persisted state
    me        int                 // this peer's index into peers[]
    // state a Raft server must maintain.
    state     State
    //Persistent state on all servers:(Updated on stable storage before responding to RPCs)
    currentTerm int    "latest term server has seen (initialized to 0 increases monotonically)"
    votedFor    int    "candidateId that received vote in current term (or null if none)"
    log         []Log  "log entries;(first index is 1)"
    //log compaction
    lastIncludedIndex   int "the snapshot replaces all entries up through and including this index"
    lastIncludedTerm    int "term of lastIncludedIndex"

    //Volatile state on all servers:每个服务器都有的数据结构
    commitIndex int    "已经复制的最新日志条目"
    lastApplied int    "已经提交的最新日志条目"

    //Volatile state on leaders：(Reinitialized after election)
    nextIndex   []int  "leader发给服务器的下一个日志条目"
    matchIndex  []int  "leader已知每个服务器已经复制的最高日志条目"

    //channel
    applyCh     chan ApplyMsg 
    killCh      chan bool
    //handle rpc
    voteCh      chan bool
    appendLogCh chan bool

}

// RequestVote RPC arguments structure.用于请求投票。

type RequestVoteArgs struct {

    Term            int "candidate’s term"
    CandidateId     int "candidate requesting vote"
    LastLogIndex    int "index of candidate’s last log entry (§5.4)"
    LastLogTerm     int "term of candidate’s last log entry (§5.4)"
		
}

// RequestVote RPC reply structure. 用于返回给请求投票的candidate!

type RequestVoteReply struct {

    Term        int  "currentTerm, for candidate to update itself"
    VoteGranted bool "true means candidate received vote"
		
}

对于每一个不同身份的服务器，定义一个循环来持续监听channel

func Make(peers []*labrpc.ClientEnd, me int,
    persister *Persister, applyCh chan ApplyMsg) *Raft {
		
    rf := &Raft{}
    rf.peers = peers
    rf.persister = persister
    rf.me = me

    rf.state = Follower
    rf.currentTerm = 0
    rf.votedFor = NULL
    rf.log = make([]Log,1) //(first index is 1)

    rf.commitIndex = 0
    rf.lastApplied = 0

    rf.applyCh = applyCh
    //because gorountne only send the chan to below goroutine,to avoid block, need 1 buffer
    rf.voteCh = make(chan bool,1)
    rf.appendLogCh = make(chan bool,1)
    rf.killCh = make(chan bool,1)
    // initialize from state persisted before a crash
    rf.readPersist(persister.ReadRaftState())

    //because from hint The tester requires that the leader send heartbeat RPCs no more than ten times per second.
    heartbeatTime := time.Duration(100) * time.Millisecond

    //from hint :You'll need to write code that takes actions periodically or after delays in time.
    //  The easiest way to do this is to create a goroutine with a loop that calls time.Sleep().
    go func() {
        for {
            select {
            case <-rf.killCh:
                return
            default:
            }
            electionTime := time.Duration(rand.Intn(200) + 300) * time.Millisecond

            rf.mu.Lock()
            state := rf.state
            rf.mu.Unlock()

            switch state {
            case Follower, Candidate:
                select {
                case <-rf.voteCh:
                case <-rf.appendLogCh:
                case <-time.After(electionTime):
                    rf.mu.Lock()
                    rf.beCandidate() //becandidate, Reset election timer, then start election
                    rf.mu.Unlock()
                }
            case Leader:
                time.Sleep(heartbeatTime)
                rf.startAppendLog()
            }
        }
    }()
    return rf
		
}


对于channel的监听实现了Follower与Candidate之间的转换，对于leader与剩余两者的转换，Candidate向除了自己的其余服务器发送选举请求，

如果在返回的rpc中发现发现对方term大于自己便转化为follower状态；

如果收到了来自leader的日志复制请求，转化为follower；

如果收到的选票数超过半数，便更新自身状态为leader并开始日志复制的工作；

如果选举超时，便开始新一轮选举，具体选举代码如下所示；

func (rf *Raft) startElection() {

    rf.mu.Lock()
    args := RequestVoteArgs{
        rf.currentTerm,
        rf.me,
        rf.getLastLogIdx(),
        rf.getLastLogTerm(),

    };
    rf.mu.Unlock()
    var votes int32 = 1;
    for i := 0; i < len(rf.peers); i++ {
        if i == rf.me {
            continue
        }
        go func(idx int) {
            reply := &RequestVoteReply{}
            ret := rf.sendRequestVote(idx,&args,reply)

            if ret {
                rf.mu.Lock()
                defer rf.mu.Unlock()
                if reply.Term > rf.currentTerm {
                    rf.beFollower(reply.Term)
                    return
                }
                if rf.state != Candidate || rf.currentTerm != args.Term{
                    return
                }
                if reply.VoteGranted {
                    atomic.AddInt32(&votes,1)
                } //If votes received from majority of servers: become leader
                if atomic.LoadInt32(&votes) > int32(len(rf.peers) / 2) {
                    rf.beLeader()
                    rf.startAppendLog()
                    send(rf.voteCh) //after be leader, then notify 'select' goroutine will sending out heartbeats immediately
                }
            }
        }(i)
    }
		
}

第二大部分是日志复制，日志复制的相关数据结构在前文中已经定义好，具体规则有以下几点：

Raft 维护了以下属性，它们共同构成了图 3 中的日志匹配属性：

如果不同服务器日志中的两个条目具有相同的索引和任期，则它们存储相同的命令。

如果不同服务器日志中的两个条目具有相同的索引和任期，则该日志在所有前面的条目中都是相同的。

![image](https://github.com/lawlietqq/Distributed-Database-with-Raft/assets/92260319/460de3b3-ae52-43de-9595-2edd7b7ade8f)

为了使 follower 的日志与自己的一致，leader 必须找到两个日志一致的最新日志条目，删除该点之后 follower 日志中的任何条目，并将该点之后 leader 的所有条目发送给 follower，所有这些操作都是为了响应 AppendEntries RPC 执行的一致性检查而发生的。leader 为每个 follower 维护一个 nextIndex，这是 leader 将发送给该 follower 的下一个日志条目的索引，当 leader 第一次上任时，它将所有 nextIndex 值初始化为其日志中最后一个之后的索引（如上图中的11）。如果一个 follower 的 log 与 leader 的不一致，则 AppendEntries 一致性检查将在下一次 AppendEntries RPC 中失败，拒绝后 leader 递减 nextIndex 并重试 AppendEntries RPC，最终 nextIndex 将达到 leader 和 follower 日志匹配的点。发生这种情况时，AppendEntries 将成功，它会删除跟随者日志中的任何冲突条目并附加 leader 日志中的条目（如果有）。一旦 AppendEntries 成功，follower 的日志与 leader 的日志一致，并且在剩余的任期内将保持这种状态。该协议可以被优化以减少被拒绝的 AppendEntries RPC 的数。
有了这种机制，leader 在上任时不需要采取任何特殊的行动来恢复日志的一致性。它刚刚开始正常运行，follower 的日志会自动收敛以响应 AppendEntries 一致性检查失败，leader 永远不会覆盖或删除自己日志中的条目。

以上一段话是构成日志复制的核心准则，具体代码如下所示：

func (rf *Raft) startAppendLog() {

    for i := 0; i < len(rf.peers); i++ {
        if i == rf.me {
            continue
        }
        go func(idx int) {
            for {
                rf.mu.Lock();
                if rf.state != Leader {
                    rf.mu.Unlock()
                    return
                } //send initial empty AppendEntries RPCs (heartbeat) to each server
                if rf.nextIndex[idx]-rf.lastIncludedIndex < 1 { //The leader uses a new RPC called InstallSnapshot to
                    rf.sendSnapshot(idx) // followers that are too far behind
                    return
                }
                args := AppendEntriesArgs{
                    rf.currentTerm,
                    rf.me,
                    rf.getPrevLogIdx(idx),
                    rf.getPrevLogTerm(idx),
                    //If last log index ≥ nextIndex for a follower:send AppendEntries RPC with log entries starting at nextIndex
                    //nextIndex > last log index, rf.log[rf.nextIndex[idx]:] will be empty then like a heartbeat
                    append(make([]Log, 0), rf.log[rf.nextIndex[idx]-rf.lastIncludedIndex:]...),
                    rf.commitIndex,
                }
                rf.mu.Unlock()
                reply := &AppendEntriesReply{}
                ret := rf.sendAppendEntries(idx, &args, reply)
                rf.mu.Lock();
                if !ret || rf.state != Leader || rf.currentTerm != args.Term {
                    rf.mu.Unlock()
                    return
                }
                if reply.Term > rf.currentTerm { //all server rule 1 If RPC response contains term T > currentTerm:
                    rf.beFollower(reply.Term) // set currentTerm = T, convert to follower (§5.1)
                    rf.mu.Unlock()
                    return
                }
                if reply.Success { //If successful：update nextIndex and matchIndex for follower
                    rf.updateNextMatchIdx(idx, args.PrevLogIndex+len(args.Entries))
                    rf.mu.Unlock()
                    return
                } else { //If AppendEntries fails because of log inconsistency: decrement nextIndex and retry
                    tarIndex := reply.ConflictIndex //If it does not find an entry with that term
                    if reply.ConflictTerm != NULL {
                        logSize := rf.logLen() //first search its log for conflictTerm
                        for i := rf.lastIncludedIndex; i < logSize; i++ { //if it finds an entry in its log with that term,
                            if rf.getLog(i).Term != reply.ConflictTerm {
                                continue
                            }
                            for i < logSize && rf.getLog(i).Term == reply.ConflictTerm {
                                i++
                            }            //set nextIndex to be the one
                            tarIndex = i //beyond the index of the last entry in that term in its log
                        }
                    }
                    rf.nextIndex[idx] = Min(rf.logLen(),tarIndex);
                    rf.mu.Unlock()
                }
            }
        }(i)
    }
		
}

lab2通过截图如下所示：

![image](https://github.com/lawlietqq/Distributed-Database-with-Raft/assets/92260319/1d0aba46-1339-49e7-b3b3-2e2c4b9e6ca0)

Lab 3: Fault-tolerant Key/Value Service

第三个实验在raft机制的基础上，实现了容错key/value服务，支持put，append，get三种操作，后续进行补充。

![image](https://github.com/lawlietqq/Distributed-Database-with-Raft/assets/92260319/92a7013a-b0ce-4139-9365-4208816d0d72)

lab3通过截图如下所示：

![image](https://github.com/lawlietqq/Distributed-Database-with-Raft/assets/92260319/c11778fc-cc5d-4777-988a-e9195960c3a9)

Lab 4: Sharded Key/Value Service

第四个实验是和Mysql类似的主从架构，由Shard Master，Shard server组成的configuration service和replica group，在组内分别构建raft共识，后续再详细进行补充代码解释及实验思想。
lab4通过截图如下所示：

![IMG_20230624_190427](https://github.com/lawlietqq/Distributed-Database-with-Raft/assets/92260319/e1542afa-d1a2-4cb1-844a-661c6ef7577d)

![IMG_20230624_190807](https://github.com/lawlietqq/Distributed-Database-with-Raft/assets/92260319/191fca24-d5bd-4b4f-a899-2f754ac1c83d)
