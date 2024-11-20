package supabase

import (
	"fmt"

	sc "github.com/supabase-community/storage-go"
)

func NewSupabaseClient(bucket_id, api_key string) *sc.Client {
	api_url := fmt.Sprintf("https://%s.supabase.co/storage/v1", bucket_id)
	return sc.NewClient(api_url, api_key, nil)
}
