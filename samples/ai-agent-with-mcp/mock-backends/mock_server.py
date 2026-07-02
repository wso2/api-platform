"""
Mock backends for Scenario 4 — Multi-Server AI Agent Demo
WSO2 Bijira API Platform Cloud

Three APIs served under one FastAPI app:
  /crm/...          — CRM API (customer profiles)
  /orders/...       — Order Management API
  /kb/...           — Knowledge Base API (article search)

Run:
  pip install fastapi uvicorn
  uvicorn mock_server:app --reload --port 8000

URLs to use when creating API proxies in Bijira:
  CRM backend    →  http://localhost:8000/crm
  Orders backend →  http://localhost:8000/orders
  KB backend     →  http://localhost:8000/kb
"""

from fastapi import FastAPI, HTTPException, Query
from fastapi.responses import JSONResponse
from typing import Optional
import datetime

app = FastAPI(title="Scenario 4 Mock Backends", version="1.0.0")


# ─────────────────────────────────────────────────────────────────────────────
# CRM API  —  /crm
# ─────────────────────────────────────────────────────────────────────────────

CUSTOMERS = {
    "C-4821": {
        "id": "C-4821",
        "name": "John Smith",
        "email": "john.smith@example.com",
        "phone": "+1-555-0182",
        "account_status": "active",
        "tier": "gold",
        "address": {
            "street": "142 Maple Avenue",
            "city": "Portland",
            "state": "OR",
            "zip": "97201",
        },
        "latest_order_id": "O-9901",
        "member_since": "2021-03-14",
    },
    "C-1001": {
        "id": "C-1001",
        "name": "Priya Nair",
        "email": "priya.nair@example.com",
        "phone": "+1-555-0291",
        "account_status": "active",
        "tier": "silver",
        "address": {
            "street": "88 Pine Street",
            "city": "Seattle",
            "state": "WA",
            "zip": "98101",
        },
        "latest_order_id": "O-9855",
        "member_since": "2022-07-22",
    },
    "C-2200": {
        "id": "C-2200",
        "name": "Marco Russo",
        "email": "marco.russo@example.com",
        "phone": "+1-555-0374",
        "account_status": "suspended",
        "tier": "basic",
        "address": {
            "street": "5 Harbor Road",
            "city": "Boston",
            "state": "MA",
            "zip": "02108",
        },
        "latest_order_id": "O-9712",
        "member_since": "2020-11-05",
    },
}


@app.get("/crm/customers/{customer_id}", tags=["CRM"])
def get_customer(customer_id: str):
    """
    Retrieve the full profile of a customer by their ID.
    Use when you need to find a customer's name, contact details, or account status.
    """
    customer = CUSTOMERS.get(customer_id)
    if not customer:
        raise HTTPException(status_code=404, detail=f"Customer {customer_id} not found")
    return customer


@app.get("/crm/customers", tags=["CRM"])
def list_customers(tier: Optional[str] = None, status: Optional[str] = None):
    """
    List customers, optionally filtered by tier or account status.
    """
    results = list(CUSTOMERS.values())
    if tier:
        results = [c for c in results if c["tier"] == tier]
    if status:
        results = [c for c in results if c["account_status"] == status]
    return {"customers": results, "total": len(results)}


# ─────────────────────────────────────────────────────────────────────────────
# Orders API  —  /orders
# ─────────────────────────────────────────────────────────────────────────────

ORDERS = {
    "O-9901": {
        "id": "O-9901",
        "customer_id": "C-4821",
        "status": "in_transit",
        "status_label": "In Transit",
        "carrier": "FedEx",
        "tracking_number": "794644774888",
        "items": [
            {"sku": "PRD-881", "name": "Wireless Headphones", "qty": 1, "unit_price": 89.99},
            {"sku": "PRD-204", "name": "USB-C Cable (3-pack)", "qty": 2, "unit_price": 14.99},
        ],
        "subtotal": 119.97,
        "shipping": 0.00,
        "total": 119.97,
        "ordered_at": "2026-05-28T10:14:00Z",
        "estimated_delivery": "2026-06-07",
        "can_cancel": False,
        "can_return": False,
        "return_eligible_from": "2026-06-07",
        "return_deadline": "2026-07-07",
    },
    "O-9855": {
        "id": "O-9855",
        "customer_id": "C-1001",
        "status": "delivered",
        "status_label": "Delivered",
        "carrier": "UPS",
        "tracking_number": "1Z9999W99999999999",
        "items": [
            {"sku": "PRD-540", "name": "Mechanical Keyboard", "qty": 1, "unit_price": 149.00},
        ],
        "subtotal": 149.00,
        "shipping": 5.99,
        "total": 154.99,
        "ordered_at": "2026-05-20T08:30:00Z",
        "estimated_delivery": "2026-05-25",
        "delivered_at": "2026-05-25T14:22:00Z",
        "can_cancel": False,
        "can_return": True,
        "return_eligible_from": "2026-05-25",
        "return_deadline": "2026-06-24",
    },
    "O-9712": {
        "id": "O-9712",
        "customer_id": "C-2200",
        "status": "processing",
        "status_label": "Processing",
        "carrier": None,
        "tracking_number": None,
        "items": [
            {"sku": "PRD-102", "name": "Laptop Stand", "qty": 1, "unit_price": 49.99},
        ],
        "subtotal": 49.99,
        "shipping": 4.99,
        "total": 54.98,
        "ordered_at": "2026-06-01T16:45:00Z",
        "estimated_delivery": "2026-06-08",
        "can_cancel": True,
        "can_return": False,
    },
}


@app.get("/orders/orders/{order_id}", tags=["Orders"])
def get_order_status(order_id: str):
    """
    Retrieve the current status, shipping carrier, and estimated delivery date of
    an order by order ID. Use when the user asks about the status, shipping,
    or delivery of an order.
    """
    order = ORDERS.get(order_id)
    if not order:
        raise HTTPException(status_code=404, detail=f"Order {order_id} not found")
    return order


@app.get("/orders/customers/{customer_id}/orders", tags=["Orders"])
def get_customer_orders(customer_id: str, status: Optional[str] = None):
    """
    List all orders for a customer. Optionally filter by status
    (processing, in_transit, delivered, cancelled).
    """
    results = [o for o in ORDERS.values() if o["customer_id"] == customer_id]
    if status:
        results = [o for o in results if o["status"] == status]
    if not results:
        raise HTTPException(status_code=404, detail=f"No orders found for customer {customer_id}")
    return {"orders": results, "total": len(results)}


# ─────────────────────────────────────────────────────────────────────────────
# Knowledge Base API  —  /kb
# ─────────────────────────────────────────────────────────────────────────────

ARTICLES = [
    {
        "id": "KB-001",
        "title": "Standard Return Policy",
        "category": "returns",
        "tags": ["return", "refund", "policy", "30 days"],
        "summary": "Our standard return policy allows returns within 30 days of delivery for most items.",
        "body": (
            "Customers may return most items within 30 days of delivery for a full refund. "
            "Items must be in their original condition and packaging. "
            "To initiate a return, log in to your account and navigate to Order History, "
            "select the order, and click 'Return Items'. A prepaid return label will be emailed "
            "to you within 24 hours. Refunds are processed within 5–7 business days after we "
            "receive the returned item."
        ),
        "url": "https://help.example.com/returns/standard-policy",
        "last_updated": "2026-04-01",
    },
    {
        "id": "KB-002",
        "title": "Non-Returnable Items",
        "category": "returns",
        "tags": ["return", "exclusions", "non-returnable", "final sale"],
        "summary": "Certain item categories are not eligible for return.",
        "body": (
            "The following items cannot be returned: digital downloads, gift cards, "
            "perishable goods, personalized/custom items, and items marked 'Final Sale'. "
            "For hygiene reasons, earbuds and in-ear headphones may only be returned if "
            "unopened. If you received a defective item, contact support within 48 hours "
            "of delivery regardless of the return window."
        ),
        "url": "https://help.example.com/returns/exclusions",
        "last_updated": "2026-04-01",
    },
    {
        "id": "KB-003",
        "title": "Refund Timelines",
        "category": "returns",
        "tags": ["refund", "timeline", "credit card", "processing"],
        "summary": "How long refunds take to appear on your account.",
        "body": (
            "After we receive your return, refunds are processed within 5–7 business days. "
            "Credit card refunds may take an additional 3–5 business days to appear on your "
            "statement depending on your card issuer. Original shipping charges are not refunded "
            "unless the return is due to our error. Store credit refunds are instant once approved."
        ),
        "url": "https://help.example.com/returns/refund-timelines",
        "last_updated": "2026-03-15",
    },
    {
        "id": "KB-004",
        "title": "Shipping & Delivery Overview",
        "category": "shipping",
        "tags": ["shipping", "delivery", "carrier", "tracking"],
        "summary": "How we ship orders and how to track your package.",
        "body": (
            "Standard shipping takes 5–7 business days. Expedited shipping (2–3 days) is "
            "available at checkout. Once your order ships, you will receive a tracking number "
            "by email. You can also track your order in your account under Order History. "
            "We ship via FedEx, UPS, and USPS depending on your location and order weight."
        ),
        "url": "https://help.example.com/shipping/overview",
        "last_updated": "2026-02-10",
    },
    {
        "id": "KB-005",
        "title": "Gold Tier Membership Benefits",
        "category": "membership",
        "tags": ["gold", "membership", "benefits", "loyalty"],
        "summary": "Perks and benefits for Gold tier members.",
        "body": (
            "Gold tier members enjoy: free standard shipping on all orders, extended 45-day "
            "return window (vs standard 30 days), early access to sales and new products, "
            "a dedicated support line, and 2x loyalty points on all purchases. "
            "Gold status is earned by spending $500 or more in a 12-month period."
        ),
        "url": "https://help.example.com/membership/gold-benefits",
        "last_updated": "2026-01-20",
    },
    {
        "id": "KB-006",
        "title": "Damaged or Defective Items",
        "category": "returns",
        "tags": ["damaged", "defective", "broken", "replacement", "return"],
        "summary": "What to do if you receive a damaged or defective item.",
        "body": (
            "If your item arrives damaged or defective, contact our support team within "
            "48 hours of delivery. Please include your order number and a photo of the damage. "
            "We will arrange a free return and send a replacement at no charge, or issue a full "
            "refund if the item is out of stock. You do not need to return defective items in "
            "their original packaging."
        ),
        "url": "https://help.example.com/returns/damaged-items",
        "last_updated": "2026-04-12",
    },
]


def score_article(article: dict, query: str) -> float:
    """Simple keyword relevance score."""
    q = query.lower()
    score = 0.0
    for tag in article["tags"]:
        if tag in q or any(w in tag for w in q.split()):
            score += 2.0
    if any(w in article["title"].lower() for w in q.split()):
        score += 3.0
    if any(w in article["summary"].lower() for w in q.split()):
        score += 1.0
    if any(w in article["body"].lower() for w in q.split()):
        score += 0.5
    return score


@app.get("/kb/search", tags=["Knowledge Base"])
def search_kb(
    q: str = Query(..., description="Search query — plain natural language or keywords"),
    limit: int = Query(3, ge=1, le=10, description="Max number of results to return"),
    category: Optional[str] = Query(None, description="Filter by category: returns, shipping, membership"),
):
    """
    Search the internal knowledge base for articles matching a query.
    Use when the user asks about policies, procedures, FAQs, or any topic
    that may be documented internally — especially return policies, shipping,
    membership benefits, and handling defective items.
    """
    candidates = ARTICLES
    if category:
        candidates = [a for a in candidates if a["category"] == category]

    scored = [(score_article(a, q), a) for a in candidates]
    scored.sort(key=lambda x: x[0], reverse=True)
    results = [a for score, a in scored if score > 0][:limit]

    return {
        "query": q,
        "total_results": len(results),
        "articles": [
            {
                "id": a["id"],
                "title": a["title"],
                "category": a["category"],
                "summary": a["summary"],
                "body": a["body"],
                "url": a["url"],
            }
            for a in results
        ],
    }


@app.get("/kb/articles/{article_id}", tags=["Knowledge Base"])
def get_article(article_id: str):
    """Retrieve a specific knowledge base article by its ID."""
    article = next((a for a in ARTICLES if a["id"] == article_id), None)
    if not article:
        raise HTTPException(status_code=404, detail=f"Article {article_id} not found")
    return article


# ─────────────────────────────────────────────────────────────────────────────
# Health check
# ─────────────────────────────────────────────────────────────────────────────

@app.get("/health", tags=["Meta"])
def health():
    return {"status": "ok", "apis": ["crm", "orders", "kb"]}


@app.get("/", tags=["Meta"])
def root():
    return {
        "message": "Scenario 4 Mock Backends — WSO2 Bijira Demo",
        "docs": "/docs",
        "openapi": "/openapi.json",
        "endpoints": {
            "crm": "/crm/customers/{id}",
            "orders": "/orders/orders/{id}",
            "kb": "/kb/search?q=...",
        },
    }
