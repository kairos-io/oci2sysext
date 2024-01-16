package main

import (
	"archive/tar"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	archive "github.com/containerd/containerd/archive"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/wagoodman/dive/dive/filetree"
	dive "github.com/wagoodman/dive/dive/image/docker"
)

// PoC for generating sysext images from docker images
// We need a wrapper to add some sugar to the UX
// (such as the user just provides a Dockerfile, or a container image built out from the dockerfile)
func getAddedFiles(f *os.File) ([]string, error) {

	archive, err := dive.NewImageArchive(f)
	if err != nil {
		return []string{}, err
	}
	i, err := archive.ToImage()
	if err != nil {
		return []string{}, err
	}

	result, err := i.Analyze()
	if err != nil {
		return []string{}, err
	}

	addedFiles := []string{}
	if len(result.Layers) < 2 {
		return []string{}, fmt.Errorf("not enough layers")
	}
	for _, l := range result.Layers[1:] {
		//fmt.Println(l.String())
		l.Tree.VisitDepthChildFirst(func(node *filetree.FileNode) error {
			//fmt.Println(l.String(), node.Data.DiffType.String())
			addedFiles = append(addedFiles, node.Data.FileInfo.Path)
			// TODO: here it shows as unmodified, but it should rather not be like that (?)
			if node.Data.DiffType == filetree.Added {
				fmt.Println(node.Data.DiffType.String())
				//	addedFiles = append(addedFiles, node.Data.FileInfo.Path)

			}

			return nil
		}, nil)
	}

	return addedFiles, nil
}

func extractDelta(img string) string {
	var err error
	var ff *os.File

	ff, err = os.Open(img)
	if err != nil {
		panic(err)
	}

	defer ff.Close()
	addedFiles, err := getAddedFiles(ff)
	if err != nil {
		panic(err)
	}

	ff.Seek(0, 0) // rewind the file

	dstImage, err := tarball.ImageFromPath(img, nil)
	if err != nil {
		panic(err)
	}

	// TODO: optimize, this is O(n^2)
	extractor := func(h *tar.Header) (bool, error) {
		for _, f := range addedFiles {
			if h.Name == f {
				return true, nil
			}
		}
		return false, nil
	}

	t, err := os.MkdirTemp("", "oci2sysext")
	if err != nil {
		panic(err)
	}
	sanityCheck(t)

	_, err = archive.Apply(context.Background(), t, mutate.Extract(dstImage), archive.WithFilter(extractor))
	if err != nil {
		panic(err)
	}
	return t
}

var allowedPaths []string = []string{
	"usr",
	"opt",
}

const attentionText = "[!] Attention!! \n\t"

func attention(s ...any) {
	fmt.Println(attentionText + fmt.Sprint(s...))
}

func analyzeRootFS(tmpdir string) {
	// list of the directories in tmpdir
	entries, err := os.ReadDir(tmpdir)
	if err != nil {
		panic(err)
	}

	for _, entry := range entries {
		if entry.IsDir() && !slices.Contains(allowedPaths, entry.Name()) {
			attention(entry.Name(), "is not a valid directory")
		}
	}

	etcFound := false
	// if there is /etc and is meant to be merged
	// warn the user that extension should be placed in /var/lib/confexts/
	if _, err := os.Stat(tmpdir + "/etc"); err == nil {
		etcFound = true
		attention("Found /etc dir in tmpdir, this is ignored unless the extension is placed in /var/lib/confexts/ (but only /etc will be merged then)")
	}

	// if both /etc and /usr is present, warn the user
	if _, err := os.Stat(tmpdir + "/usr"); err == nil && etcFound {
		attention("Both /etc and /usr found, this is not supported by systemd. The extension can be either shipping configurations or binaries/other files, not both.")
	}

	fmt.Println("Generating systemd-extension with the following files:")
	filepath.Walk(tmpdir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		f := strings.ReplaceAll(path, tmpdir+"/", "")
		fmt.Println("- ", f)
		return nil
	})
}

func createExtensionsMetadataFiles(tmpdir string, imgName string) {
	err := os.MkdirAll(tmpdir+"/usr/lib/extension-release.d", 0755)
	if err != nil {
		panic(err)
	}
	err = os.WriteFile(tmpdir+"/usr/lib/extension-release.d/extension-release."+imgName, []byte("ID=_any"), 0644)
	if err != nil {
		panic(err)
	}
}

func genSquashFS(tmpdir string, imgName string) {
	fmt.Println("Writing squashfs", imgName+".raw")
	out, err := exec.Command("mksquashfs", tmpdir, imgName+".raw").CombinedOutput()
	if err != nil {
		fmt.Println(string(out))
		panic(err)
	}
}

func sanityCheck(dir string) {
	// Sanity checks
	switch dir {
	case "/":
		panic("dir is /")
	case "":
		panic("dir is empty")
	}
}

// Roughly what oci2sysext does is:
// Load the container image tarball
// Find out all the files that are added in the container image
// Extract with containerd only the files needeed from the image.
// Convert it to a squashfs for systemd-sysext
func main() {
	img := os.Args[1]
	imgName := os.Args[2]

	tmpdir := extractDelta(img)

	// Sanity checks
	sanityCheck(tmpdir)

	defer os.RemoveAll(tmpdir)

	analyzeRootFS(tmpdir)
	createExtensionsMetadataFiles(tmpdir, imgName)
	genSquashFS(tmpdir, imgName)
}
