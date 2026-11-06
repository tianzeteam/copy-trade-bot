// Package pumpV3 实现 Pump.fun 2026 年升级后的内盘（bonding curve）协议。
//
// # 命名说明
//
// 协议侧把新的统一交易指令命名为 buy_v2 / sell_v2 / buy_exact_quote_in_v2，
// 但本仓库已存在两代绑定：pump（V1，SPL Token）与 pumpV2（Token-2022）。
// 这两代区分的是 base token program，而协议新接口区分的是指令世代，
// 两者不是同一维度。为避免歧义，本包取名 pumpV3。
//
// # 相比 pump / pumpV2 的关键变化
//
//  1. 统一账户接口：buy_v2 固定 27 个账户、sell_v2 固定 26 个账户，
//     不再有「按代币类型决定传哪些可选账户」的分支，SOL 与 USDC 配对币走同一条路径。
//  2. 引入 quote mint 维度：BondingCurve 新增 quote_mint 字段，
//     TradeEvent 新增 quote_mint / quote_amount / virtual_quote_reserves / real_quote_reserves。
//     非 SOL 配对币的价格单位由 quote mint 决定（USDC 为 6 位小数），
//     不能再按 SOL 的 9 位小数换算。
//  3. 手续费改为动态分档：由独立的 pump_fees 程序按代币市值（lamports 计）
//     查 FeeConfig.fee_tiers 得出 protocol / creator / lp 费率。
//     用户实际承担的成本 = 曲线成本 + 协议费 + creator 费；
//     buyback 费从协议费内部分账，不另行叠加（已与官方 TS / Rust SDK 核对）。
//     下单时的 max_sol_cost / min_sol_output 按上述三项计算后再叠加滑点。
//  4. fee recipient 分裂为三类共 24 个地址：普通币用 normal，
//     mayhem 模式币必须用 reserved，所有币都必须额外传一个 buyback recipient。
//  5. 新增 holder rewards 币：creator fee 归持有人而非创建者钱包，
//     交易接口不变，仅 TradeEvent 增加 holder_rewards_bps / holder_rewards。
//  6. cashback 模式已废弃：create_v2 拒绝 is_cashback_enabled = true，
//     存量 cashback 币仍可交易与领取。
//
// 字段重命名 real_sol_reserves → real_quote_reserves、
// virtual_sol_reserves → virtual_quote_reserves 只影响语义命名，
// borsh 按位置编解码，因此旧账户数据仍可正确读出。
//
// # 参考
//
// 官方文档与 IDL：https://github.com/pump-fun/pump-public-docs
// 本包依据 idl/pump.json 与 idl/pump_fees.json（2026-09-14）手工编写，
// 非 anchor-go 生成，修改前请先比对官方 IDL。
package pumpV3
