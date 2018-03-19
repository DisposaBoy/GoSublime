package mg

type KVStore interface {
	Put(k interface{}, v interface{})
	Get(k interface{}) interface{}
	Del(k interface{})
}
