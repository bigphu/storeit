// Tên ngắn cho schema hay dùng
import type { components as Identity } from './identity'
import type { components as Inventory } from './inventory'

type I = Identity['schemas']
type V = Inventory['schemas']

export type Account = I['Account']
export type AccountDetail = I['AccountDetail']
export type AccountListItem = I['AccountListItem']
export type Role = I['Role']
export type RoleSummary = I['RoleSummary']
export type Permission = I['Permission']
export type Me = I['Me']
export type SessionResponse = I['SessionResponse']

export type AssetType = V['AssetType']
export type Attribute = V['Attribute']
export type Option = V['Option']
export type DataType = V['DataType']
export type Status = V['Status']
export type StatusKind = V['StatusKind']
export type AssetListItem = V['AssetListItem']
export type AssetDetail = V['AssetDetail']
export type AttributeValue = V['AttributeValue']
export type ExportLayout = V['ExportLayout']
export type ExportColumn = V['ExportColumn']
export type ExportFilters = V['ExportFilters']
export type ExportRequest = V['ExportRequest']
export type ExportProfile = V['ExportProfile']
