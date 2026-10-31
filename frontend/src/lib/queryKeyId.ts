// keyId: id của query nằm trong khoá (ở vị trí index). queryFn phải đọc id từ khoá, không từ
// ref của trang: khi một thay đổi làm tải lại mọi query (vd giao tài sản tải lại mọi tài sản),
// ref có thể đã đổi hay rỗng (trang ở tab nền, ngăn vừa đóng) trong khi query vẫn mang khoá
// cũ, và gọi API với id rỗng ra '/assets/{assetID}' (400 "invalid parameters"). Khoá không có
// id thì không gọi API: NoQueryId không thử lại, không hiện toast (lib/query.ts)
export class NoQueryId extends Error {}

export function keyId(queryKey: readonly unknown[], index: number): string {
  const id = queryKey[index]
  if (typeof id !== 'string' || id === '') throw new NoQueryId(`query ${JSON.stringify(queryKey)} has no id`)
  return id
}
