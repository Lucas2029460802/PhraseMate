package gitdata

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"phrasemate/internal/models"
	"phrasemate/internal/store"
)

// Sync publishes the vocabulary snapshot to the repository's data branch.
// API settings stay in the local database and are never written to git.
type Sync struct {
	root   string
	remote string
	branch string
	author string
	email  string

	opMu      sync.Mutex
	lastWords []models.Word

	bgMu        sync.Mutex
	bgRunning   bool
	bgAgain     bool
	lastPushErr error
	onStatus    func(branch, errMsg string)
	wg          sync.WaitGroup
	pushedOnce  sync.Once
}

type refSnap struct {
	exists bool
	hash   string
	words  []models.Word
}

// Discover opens the git repository that contains the working directory
// or the executable. Sync stays off when git or origin is unavailable.
func Discover() (*Sync, error) {
	if syncDisabled() {
		return nil, errors.New("PHRASEMATE_GIT_SYNC 已关闭")
	}
	if _, err := exec.LookPath("git"); err != nil {
		return nil, errors.New("未找到 git，无法写入 data 分支")
	}
	root, err := findRoot()
	if err != nil {
		return nil, err
	}
	return Open(root)
}

// Open binds an existing repository. The remote and branch come from the environment.
func Open(root string) (*Sync, error) {
	return openAt(root, envOr("PHRASEMATE_GIT_REMOTE", "origin"), envOr("PHRASEMATE_GIT_BRANCH", "data"))
}

func openAt(root, remote, branch string) (*Sync, error) {
	remote = strings.TrimSpace(remote)
	branch = strings.TrimSpace(branch)
	if !validRefName(remote) || !validRefName(branch) {
		return nil, errors.New("远程名或分支名无效")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	s := &Sync{
		root:   abs,
		remote: remote,
		branch: branch,
		author: "PhraseMate",
		email:  "phrasemate@users.noreply.github.com",
	}
	top, err := s.gitLocal("rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("不是 Git 仓库 (%s): %w", abs, err)
	}
	s.root = strings.TrimSpace(string(top))
	if _, err := s.gitLocal("remote", "get-url", s.remote); err != nil {
		return nil, fmt.Errorf("未配置远程 %s", s.remote)
	}
	if name, email, err := s.readAuthor(); err == nil {
		s.author = name
		s.email = email
	}
	return s, nil
}

// Branch is the git branch that stores words.json.
func (s *Sync) Branch() string {
	if s == nil {
		return ""
	}
	return s.branch
}

// SetStatusHandler receives the branch name and the latest push error.
func (s *Sync) SetStatusHandler(fn func(branch, errMsg string)) {
	if s == nil {
		return
	}
	s.bgMu.Lock()
	s.onStatus = fn
	s.bgMu.Unlock()
}

// Bootstrap loads words.json from the data branch, creates that branch when
// it is missing, and schedules a push of the merged snapshot.
func (s *Sync) Bootstrap(st *store.Store) error {
	if s == nil || st == nil {
		return errors.New("缺少词库")
	}
	words, parent, err := s.mergeFromGit(st)
	if err != nil {
		return err
	}
	if err := st.ReplaceAllWords(words); err != nil {
		return err
	}
	s.opMu.Lock()
	err = s.commitLocked(words, parent)
	s.opMu.Unlock()
	if err != nil {
		return err
	}
	log.Printf("生词本已写入本地分支 %s（%d 条）", s.branch, len(words))
	s.report(nil)
	s.schedulePush()
	return nil
}

// Save commits the current vocabulary and schedules a push.
func (s *Sync) Save(st *store.Store) error {
	if s == nil || st == nil {
		return errors.New("缺少词库")
	}
	s.opMu.Lock()
	words, err := st.List()
	if err != nil {
		s.opMu.Unlock()
		return err
	}
	err = s.commitLocked(words, "")
	s.opMu.Unlock()
	if err != nil {
		return err
	}
	s.report(nil)
	s.schedulePush()
	return nil
}

// Flush waits until the queued push finishes.
func (s *Sync) Flush() error {
	if s == nil {
		return nil
	}
	s.wg.Wait()
	s.bgMu.Lock()
	defer s.bgMu.Unlock()
	return s.lastPushErr
}

// Close waits for an in-flight push so the process can exit after it.
func (s *Sync) Close() {
	if s == nil {
		return
	}
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(25 * time.Second):
		log.Printf("等待推送 %s 分支超时", s.branch)
	}
}

func (s *Sync) mergeFromGit(st *store.Store) ([]models.Word, string, error) {
	localDB, err := st.List()
	if err != nil {
		return nil, "", err
	}
	hasRemote := false
	if ok, ferr := s.fetchRemote(); ferr != nil {
		log.Printf("拉取 %s 分支失败，先使用本地记录: %v", s.branch, ferr)
	} else {
		hasRemote = ok
	}
	localSnap, err := s.readSnap(s.localRef())
	if err != nil {
		return nil, "", err
	}
	var remoteSnap refSnap
	if hasRemote {
		remoteSnap, err = s.readSnap(s.remoteRef())
		if err != nil {
			return nil, "", err
		}
	}
	rel := s.relation(localSnap, remoteSnap)
	words := selectSnapshot(localDB, localSnap.words, localSnap.exists, remoteSnap.words, remoteSnap.exists, rel)
	parent := parentFor(localSnap.hash, localSnap.exists, remoteSnap.hash, remoteSnap.exists, rel)
	return words, parent, nil
}

func (s *Sync) fetchRemote() (bool, error) {
	out, err := s.gitNet(fetchTimeout, "ls-remote", "--heads", s.remote, "refs/heads/"+s.branch)
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(string(out)) == "" {
		return false, nil
	}
	refspec := fmt.Sprintf("+refs/heads/%s:%s", s.branch, s.remoteRef())
	if _, err := s.gitNet(fetchTimeout, "fetch", s.remote, refspec); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Sync) relation(local, remote refSnap) Relation {
	if !local.exists || !remote.exists {
		return relNone
	}
	if local.hash == remote.hash {
		return relEqual
	}
	localAhead, err := s.isAncestor(s.remoteRef(), s.localRef())
	if err != nil {
		log.Printf("比较 %s 分支失败: %v", s.branch, err)
		return relDiverged
	}
	if localAhead {
		return relLocalAhead
	}
	remoteAhead, err := s.isAncestor(s.localRef(), s.remoteRef())
	if err != nil {
		return relDiverged
	}
	if remoteAhead {
		return relRemoteAhead
	}
	return relDiverged
}

func (s *Sync) isAncestor(ancestor, descendant string) (bool, error) {
	_, err := s.gitLocal("merge-base", "--is-ancestor", ancestor, descendant)
	if err == nil {
		return true, nil
	}
	if exitCode(err) == 1 {
		return false, nil
	}
	return false, err
}

func (s *Sync) readSnap(ref string) (refSnap, error) {
	hash, ok, err := s.optionalHash(ref)
	if err != nil || !ok {
		return refSnap{}, err
	}
	out, err := s.gitLocal("show", ref+":"+wordsFile)
	if err != nil {
		if isMissingPath(err) {
			return refSnap{exists: true, hash: hash}, nil
		}
		return refSnap{}, err
	}
	words, err := unmarshalWords(out)
	if err != nil {
		return refSnap{}, fmt.Errorf("解析 %s 失败: %w", wordsFile, err)
	}
	return refSnap{exists: true, hash: hash, words: words}, nil
}

func (s *Sync) optionalHash(ref string) (string, bool, error) {
	out, err := s.gitLocal("rev-parse", "--verify", "--quiet", ref)
	if err != nil {
		if exitCode(err) == 1 {
			return "", false, nil
		}
		return "", false, err
	}
	hash := strings.TrimSpace(string(out))
	if hash == "" {
		return "", false, nil
	}
	return hash, true, nil
}

func (s *Sync) commitLocked(words []models.Word, parent string) error {
	words = prepareWords(words)
	body, err := marshalWords(words)
	if err != nil {
		return err
	}
	s.lastWords = append([]models.Word(nil), words...)

	blob, err := s.hashObject(body)
	if err != nil {
		return err
	}
	tree, err := s.mkTree(blob)
	if err != nil {
		return err
	}
	localHash, hasLocal, err := s.optionalHash(s.localRef())
	if err != nil {
		return err
	}
	if parent == "" && hasLocal {
		parent = localHash
	}
	if parent != "" {
		parentTree, err := s.revParse(parent + "^{tree}")
		if err != nil {
			return err
		}
		if parentTree == tree {
			if !hasLocal || localHash != parent {
				old := ""
				if hasLocal {
					old = localHash
				}
				return s.updateRef(s.localRef(), parent, old)
			}
			return nil
		}
	}
	commit, err := s.commitTree(tree, parent)
	if err != nil {
		return err
	}
	old := ""
	if hasLocal {
		old = localHash
	}
	return s.updateRef(s.localRef(), commit, old)
}

func (s *Sync) hashObject(body []byte) (string, error) {
	out, err := s.gitStdin(body, "hash-object", "-w", "--stdin")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (s *Sync) mkTree(blob string) (string, error) {
	in := fmt.Sprintf("100644 blob %s\t%s\n", blob, wordsFile)
	out, err := s.gitStdin([]byte(in), "mktree")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (s *Sync) commitTree(tree, parent string) (string, error) {
	args := []string{"commit-tree", tree}
	if parent != "" {
		args = append(args, "-p", parent)
	}
	args = append(args, "-m", "更新生词本")
	out, err := s.gitLocal(args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (s *Sync) updateRef(ref, newHash, oldHash string) error {
	args := []string{"update-ref", ref, newHash}
	if oldHash != "" {
		args = append(args, oldHash)
	}
	_, err := s.gitLocal(args...)
	return err
}

func (s *Sync) revParse(ref string) (string, error) {
	out, err := s.gitLocal("rev-parse", "--verify", ref)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (s *Sync) schedulePush() {
	s.bgMu.Lock()
	defer s.bgMu.Unlock()
	if s.bgRunning {
		s.bgAgain = true
		return
	}
	s.bgRunning = true
	s.wg.Add(1)
	go s.pushLoop()
}

func (s *Sync) pushLoop() {
	defer s.wg.Done()
	for {
		err := s.pushBranch()
		s.bgMu.Lock()
		s.lastPushErr = err
		again := s.bgAgain
		s.bgAgain = false
		if !again {
			s.bgRunning = false
		}
		s.bgMu.Unlock()
		if err != nil {
			log.Printf("推送 %s 分支失败: %v", s.branch, err)
		} else {
			s.pushedOnce.Do(func() {
				log.Printf("已推送生词到 %s 分支", s.branch)
			})
		}
		s.report(err)
		if !again {
			return
		}
	}
}

func (s *Sync) pushBranch() error {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	err := s.tryPushLocked()
	if err == nil || !isNonFastForward(err) {
		return err
	}
	ok, ferr := s.fetchRemote()
	if ferr != nil {
		return fmt.Errorf("%w（重新拉取失败: %v）", err, ferr)
	}
	if !ok {
		return err
	}
	remoteHash, hasRemote, herr := s.optionalHash(s.remoteRef())
	if herr != nil {
		return herr
	}
	if !hasRemote {
		return err
	}
	if rerr := s.commitLocked(s.lastWords, remoteHash); rerr != nil {
		return rerr
	}
	return s.tryPushLocked()
}

func (s *Sync) tryPushLocked() error {
	if _, ok, err := s.optionalHash(s.localRef()); err != nil || !ok {
		return err
	}
	spec := s.localRef() + ":" + s.localRef()
	_, err := s.gitNet(pushTimeout, "push", s.remote, spec)
	return err
}

func (s *Sync) report(err error) {
	s.bgMu.Lock()
	fn := s.onStatus
	branch := s.branch
	s.bgMu.Unlock()
	if fn == nil {
		return
	}
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	fn(branch, msg)
}

func (s *Sync) readAuthor() (string, string, error) {
	out, err := s.gitLocal("log", "-1", "--format=%an%x1f%ae")
	if err != nil {
		return "", "", err
	}
	name, email, ok := strings.Cut(strings.TrimSpace(string(out)), "\x1f")
	name = strings.TrimSpace(name)
	email = strings.TrimSpace(email)
	if !ok || name == "" || email == "" {
		return "", "", errors.New("仓库还没有提交记录")
	}
	return name, email, nil
}

func (s *Sync) localRef() string {
	return "refs/heads/" + s.branch
}

func (s *Sync) remoteRef() string {
	return "refs/remotes/" + s.remote + "/" + s.branch
}

func syncDisabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("PHRASEMATE_GIT_SYNC"))) {
	case "0", "false", "no", "off":
		return true
	default:
		return false
	}
}

func findRoot() (string, error) {
	if p := strings.TrimSpace(os.Getenv("PHRASEMATE_REPO")); p != "" {
		abs, err := filepath.Abs(p)
		if err != nil {
			return "", err
		}
		return abs, nil
	}
	if root, err := gitTop("."); err == nil {
		return root, nil
	}
	if exe, err := os.Executable(); err == nil {
		if root, err := gitTop(filepath.Dir(exe)); err == nil {
			return root, nil
		}
	}
	return "", errors.New("当前不在 Git 仓库中，生词只保存在本地数据库")
}

func gitTop(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	configureCmd(cmd)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	root := strings.TrimSpace(string(out))
	if root == "" {
		return "", errors.New("找不到仓库根目录")
	}
	return root, nil
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func validRefName(name string) bool {
	if name == "" || strings.HasPrefix(name, "-") || strings.HasPrefix(name, "/") || strings.HasSuffix(name, "/") || strings.Contains(name, "..") {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '.' || r == '_' || r == '-' || r == '/':
		default:
			return false
		}
	}
	return true
}

func isMissingPath(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "does not exist") || strings.Contains(msg, "exists on disk, but not")
}

func isNonFastForward(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "non-fast-forward") || strings.Contains(msg, "fetch first") || strings.Contains(msg, "[rejected]")
}
