# ringbuffer

A golang is a thread-safe circular buffer library.

## Usage

```golang
import "github.com/mxmauro/ringbuffer"

func main() {
	rb := ringbuffer.New(1024)

	// ....

	n, err := rb.Write(buf)

	// ....

	n, err := rb.Read(buf)
}
```

`RingBuffer` implements the `io.ReadWriter` interface and contains other helper methods like `Peek` and `Find` to view
data in advance and find data in the buffer.

Simultaneous read and write to the ring buffer is possible because the implementation contains a synchronization mutex.

##### NOTE:

* The buffer size grows as needed but never shrinks. Adjust the `growSize` parameter depending on your application
  requirements.

## LICENSE

MIT. See the [LICENSE](/LICENSE) file for details.
