import { createFileRoute } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { useAuthStore } from '../stores/auth'
import { apiGet } from '../api/client'

export const Route = createFileRoute('/orders')({
  component: OrdersPage,
})

interface OrderInfo {
  order_no: string
  status: string
  amount_credits: number
  created_at: string
}

function OrdersPage() {
  const token = useAuthStore((s) => s.token)
  const [orders, setOrders] = useState<OrderInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!token) return
    apiGet<OrderInfo[]>('/market/orders', token)
      .then(setOrders)
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false))
  }, [token])

  return (
    <div className="p-6 space-y-4">
      <h1 className="text-2xl font-bold">Orders</h1>

      {loading && <p className="text-muted-foreground">Loading...</p>}
      {error && <p className="text-red-500 text-sm">{error}</p>}

      {!loading && !error && orders.length === 0 && (
        <p className="text-muted-foreground">No orders yet.</p>
      )}

      {orders.length > 0 && (
        <div className="border rounded-md overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-muted/50">
              <tr>
                <th className="text-left p-3 font-medium">Order No.</th>
                <th className="text-left p-3 font-medium">Status</th>
                <th className="text-right p-3 font-medium">Credits</th>
                <th className="text-left p-3 font-medium">Created At</th>
              </tr>
            </thead>
            <tbody>
              {orders.map((o) => (
                <tr key={o.order_no} className="border-t">
                  <td className="p-3 font-mono text-xs">{o.order_no}</td>
                  <td className="p-3">
                    <span className="text-xs px-2 py-1 rounded bg-blue-100 text-blue-700 capitalize">
                      {o.status}
                    </span>
                  </td>
                  <td className="p-3 text-right">{o.amount_credits}</td>
                  <td className="p-3 text-muted-foreground">{new Date(o.created_at).toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
