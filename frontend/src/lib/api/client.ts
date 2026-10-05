// Client có kiểu cho từng module, sinh từ OpenAPI của backend (npm run gen:api).
// Cả hai dùng authFetch: gắn bearer, gặp 401 thì refresh một lần rồi gửi lại.
import createClient from 'openapi-fetch'
import { tokens } from '@/lib/auth/tokens-instance'
import type { paths as IdentityPaths } from './identity'
import type { paths as InventoryPaths } from './inventory'

const options = { baseUrl: '/api/v1', fetch: (r: Request) => tokens.authFetch(r) }

export const identityApi = createClient<IdentityPaths>(options)
export const inventoryApi = createClient<InventoryPaths>(options)
