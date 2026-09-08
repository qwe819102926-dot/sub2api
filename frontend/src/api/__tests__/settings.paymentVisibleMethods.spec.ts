import { describe, expect, it } from 'vitest'

import {
  getPaymentVisibleMethodSourceOptions,
  normalizePaymentVisibleMethodSource,
  normalizeFixedSourceRoutingSettings,
} from '@/api/admin/settings'

describe('admin settings payment visible method helpers', () => {
  it('normalizes incomplete fixed source routing payloads', () => {
    expect(normalizeFixedSourceRoutingSettings({ enabled: true })).toEqual({
      enabled: true,
      domains: [],
      ips: [],
      routes: [],
    })
  })

  it('drops malformed fixed source routes while preserving valid ids', () => {
    expect(normalizeFixedSourceRoutingSettings({
      domains: null,
      ips: ['203.0.113.10', 123],
      routes: [
        { source_group_id: 2.9, target_group_id: 3, account_id: 4 },
        null,
      ],
    })).toEqual({
      enabled: false,
      domains: [],
      ips: ['203.0.113.10'],
      routes: [{ source_group_id: 2, target_group_id: 3, account_id: 4 }],
    })
  })

  it('normalizes aliases into canonical source keys per visible method', () => {
    expect(normalizePaymentVisibleMethodSource('alipay', 'official')).toBe('official_alipay')
    expect(normalizePaymentVisibleMethodSource('alipay', 'alipay_direct')).toBe('official_alipay')
    expect(normalizePaymentVisibleMethodSource('alipay', 'easypay')).toBe('easypay_alipay')

    expect(normalizePaymentVisibleMethodSource('wxpay', 'official')).toBe('official_wxpay')
    expect(normalizePaymentVisibleMethodSource('wxpay', 'wechat')).toBe('official_wxpay')
    expect(normalizePaymentVisibleMethodSource('wxpay', 'easypay')).toBe('easypay_wxpay')
  })

  it('rejects unknown or cross-method source values', () => {
    expect(normalizePaymentVisibleMethodSource('alipay', 'official_wxpay')).toBe('')
    expect(normalizePaymentVisibleMethodSource('wxpay', 'official_alipay')).toBe('')
    expect(normalizePaymentVisibleMethodSource('alipay', 'unknown')).toBe('')
    expect(normalizePaymentVisibleMethodSource('wxpay', null)).toBe('')
  })

  it('exposes method-scoped source options instead of arbitrary strings', () => {
    expect(getPaymentVisibleMethodSourceOptions('alipay')).toEqual([
      {
        value: '',
        labelZh: '未配置',
        labelEn: 'Not configured',
      },
      {
        value: 'official_alipay',
        labelZh: '支付宝官方',
        labelEn: 'Official Alipay',
      },
      {
        value: 'easypay_alipay',
        labelZh: '易支付支付宝',
        labelEn: 'EasyPay Alipay',
      },
      {
        value: 'jianpay_alipay',
        labelZh: '简付支付宝',
        labelEn: 'JianPay Alipay',
      },
    ])

    expect(getPaymentVisibleMethodSourceOptions('wxpay')).toEqual([
      {
        value: '',
        labelZh: '未配置',
        labelEn: 'Not configured',
      },
      {
        value: 'official_wxpay',
        labelZh: '微信官方',
        labelEn: 'Official WeChat Pay',
      },
      {
        value: 'easypay_wxpay',
        labelZh: '易支付微信',
        labelEn: 'EasyPay WeChat Pay',
      },
      {
        value: 'jianpay_wxpay',
        labelZh: '简付微信',
        labelEn: 'JianPay WeChat Pay',
      },
    ])
  })
})
