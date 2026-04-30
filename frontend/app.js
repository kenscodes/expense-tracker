/**
 * Expense Tracker — Frontend Application
 *
 * Key design decisions:
 * - Idempotency keys (UUID v4) generated per submission to prevent duplicates
 * - Double-click prevention by disabling submit during API calls
 * - Loading, error, and empty states for all data views
 * - Graceful error handling with retry support
 */

// Configuration

const API_BASE = window.location.origin;
const CATEGORIES = ['Food', 'Transport', 'Shopping', 'Entertainment', 'Bills', 'Healthcare', 'Education', 'Other'];

// UUID v4 Generator (for idempotency keys)

function generateUUID() {
    // Use crypto.randomUUID if available, otherwise fallback
    if (typeof crypto !== 'undefined' && crypto.randomUUID) {
        return crypto.randomUUID();
    }
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function (c) {
        const r = (Math.random() * 16) | 0;
        const v = c === 'x' ? r : (r & 0x3) | 0x8;
        return v.toString(16);
    });
}

// DOM Elements

const elements = {
    form: document.getElementById('expense-form'),
    submitBtn: document.getElementById('submit-btn'),
    btnText: document.querySelector('.btn-text'),
    btnLoader: document.querySelector('.btn-loader'),
    toast: document.getElementById('toast'),
    toastMessage: document.getElementById('toast-message'),

    // Form fields
    amount: document.getElementById('amount'),
    category: document.getElementById('category'),
    description: document.getElementById('description'),
    date: document.getElementById('date'),

    // Error spans
    amountError: document.getElementById('amount-error'),
    categoryError: document.getElementById('category-error'),
    descriptionError: document.getElementById('description-error'),
    dateError: document.getElementById('date-error'),

    // Controls
    filterCategory: document.getElementById('filter-category'),
    sortOrder: document.getElementById('sort-order'),

    // Display
    totalAmount: document.getElementById('total-amount'),
    expenseCount: document.getElementById('expense-count'),

    // States
    loadingState: document.getElementById('loading-state'),
    errorState: document.getElementById('error-state'),
    emptyState: document.getElementById('empty-state'),
    errorMessage: document.getElementById('error-message'),
    retryBtn: document.getElementById('retry-btn'),

    // Table
    expenseTable: document.getElementById('expense-table'),
    expenseTbody: document.getElementById('expense-tbody'),

    // Summary
    summarySection: document.getElementById('summary-section'),
    summaryList: document.getElementById('summary-list'),
};

// State

let isSubmitting = false;
let currentIdempotencyKey = null;

// API Client

async function apiRequest(method, path, body = null, headers = {}) {
    const url = `${API_BASE}${path}`;
    const config = {
        method,
        headers: {
            'Content-Type': 'application/json',
            ...headers,
        },
    };

    if (body) {
        config.body = JSON.stringify(body);
    }

    const response = await fetch(url, config);
    const data = await response.json();

    if (!response.ok) {
        throw new Error(data.error || `Request failed with status ${response.status}`);
    }

    return { data, status: response.status };
}

// Form Validation

function clearErrors() {
    elements.amountError.textContent = '';
    elements.categoryError.textContent = '';
    elements.descriptionError.textContent = '';
    elements.dateError.textContent = '';

    elements.amount.classList.remove('invalid');
    elements.category.classList.remove('invalid');
    elements.description.classList.remove('invalid');
    elements.date.classList.remove('invalid');
}

function validateForm() {
    clearErrors();
    let valid = true;

    const amount = parseFloat(elements.amount.value);
    if (!elements.amount.value || isNaN(amount) || amount <= 0) {
        elements.amountError.textContent = 'Amount must be greater than zero';
        elements.amount.classList.add('invalid');
        valid = false;
    } else if (amount > 100000000) {
        elements.amountError.textContent = 'Amount exceeds maximum (₹10,00,00,000)';
        elements.amount.classList.add('invalid');
        valid = false;
    }

    if (!elements.category.value) {
        elements.categoryError.textContent = 'Please select a category';
        elements.category.classList.add('invalid');
        valid = false;
    }

    if (!elements.description.value.trim()) {
        elements.descriptionError.textContent = 'Description is required';
        elements.description.classList.add('invalid');
        valid = false;
    } else if (elements.description.value.trim().length > 500) {
        elements.descriptionError.textContent = 'Description must be 500 characters or fewer';
        elements.description.classList.add('invalid');
        valid = false;
    }

    if (!elements.date.value) {
        elements.dateError.textContent = 'Date is required';
        elements.date.classList.add('invalid');
        valid = false;
    }

    return valid;
}

// Form Submission

async function handleSubmit(e) {
    e.preventDefault();

    // Prevent double submission
    if (isSubmitting) return;

    if (!validateForm()) return;

    isSubmitting = true;
    setSubmitLoading(true);

    // Generate a new idempotency key for this submission attempt.
    // If the user clicks again (double-click) the same key is reused,
    // ensuring the backend only creates one expense.
    if (!currentIdempotencyKey) {
        currentIdempotencyKey = generateUUID();
    }

    const payload = {
        amount: parseFloat(elements.amount.value),
        category: elements.category.value,
        description: elements.description.value.trim(),
        date: elements.date.value,
    };

    try {
        const { data, status } = await apiRequest('POST', '/api/expenses', payload, {
            'Idempotency-Key': currentIdempotencyKey,
        });

        // Success — reset form and reload list
        showToast(status === 201 ? 'Expense added successfully!' : 'Expense already recorded (duplicate prevented)', 'success');
        elements.form.reset();
        setDefaultDate();
        currentIdempotencyKey = null; // Reset for next submission

        // Reload expenses and summary
        await Promise.all([loadExpenses(), loadSummary()]);
    } catch (err) {
        showToast(err.message || 'Failed to add expense. Please try again.', 'error');
        // Keep idempotency key so retry uses the same key
    } finally {
        isSubmitting = false;
        setSubmitLoading(false);
    }
}

function setSubmitLoading(loading) {
    elements.submitBtn.disabled = loading;
    elements.btnText.hidden = loading;
    elements.btnLoader.hidden = !loading;
}

// Toast Notifications

function showToast(message, type = 'success') {
    elements.toastMessage.textContent = message;
    elements.toast.className = `toast ${type}`;
    elements.toast.hidden = false;

    // Auto-hide after 4 seconds
    setTimeout(() => {
        elements.toast.hidden = true;
    }, 4000);
}

// Load & Render Expenses

async function loadExpenses() {
    showState('loading');

    const category = elements.filterCategory.value;
    const sort = elements.sortOrder.value;

    let params = new URLSearchParams();
    if (category) params.set('category', category);
    if (sort) params.set('sort', sort);

    const queryString = params.toString();
    const path = `/api/expenses${queryString ? '?' + queryString : ''}`;

    try {
        const { data } = await apiRequest('GET', path);
        renderExpenses(data);
        updateFilterCategories();
    } catch (err) {
        showState('error', err.message || 'Failed to load expenses');
    }
}

function renderExpenses(data) {
    const { expenses, total_display, count } = data;

    // Update total
    elements.totalAmount.textContent = total_display;
    elements.expenseCount.textContent = `${count} expense${count !== 1 ? 's' : ''}`;

    if (!expenses || expenses.length === 0) {
        showState('empty');
        return;
    }

    // Build table rows
    elements.expenseTbody.innerHTML = expenses
        .map(
            (exp) => `
        <tr>
            <td class="expense-date">${formatDate(exp.date)}</td>
            <td><span class="category-badge" data-category="${escapeHtml(exp.category)}">${escapeHtml(exp.category)}</span></td>
            <td class="expense-description" title="${escapeHtml(exp.description)}">${escapeHtml(exp.description)}</td>
            <td class="text-right expense-amount">${escapeHtml(exp.amount_display)}</td>
        </tr>
    `
        )
        .join('');

    showState('data');
}

function showState(state, message = '') {
    elements.loadingState.hidden = state !== 'loading';
    elements.errorState.hidden = state !== 'error';
    elements.emptyState.hidden = state !== 'empty';
    elements.expenseTable.hidden = state !== 'data';

    if (state === 'error') {
        elements.errorMessage.textContent = message;
    }

    if (state === 'empty' || state === 'error') {
        elements.totalAmount.textContent = '₹0.00';
        elements.expenseCount.textContent = '0 expenses';
    }
}

// Category Filter (Dynamic)

async function updateFilterCategories() {
    try {
        // Fetch all expenses (unfiltered) to extract categories
        const { data } = await apiRequest('GET', '/api/expenses');
        const categories = [...new Set(data.expenses.map((e) => e.category))].sort();

        const currentValue = elements.filterCategory.value;
        elements.filterCategory.innerHTML = '<option value="">All Categories</option>';

        categories.forEach((cat) => {
            const option = document.createElement('option');
            option.value = cat;
            option.textContent = cat;
            if (cat === currentValue) option.selected = true;
            elements.filterCategory.appendChild(option);
        });
    } catch {
        // Silently fail — filter will still work with static categories
    }
}

// Category Summary

async function loadSummary() {
    try {
        const { data } = await apiRequest('GET', '/api/expenses/summary');
        renderSummary(data);
    } catch {
        elements.summarySection.hidden = true;
    }
}

function renderSummary(data) {
    const { categories, grand_total } = data;

    if (!categories || categories.length === 0) {
        elements.summarySection.hidden = true;
        return;
    }

    elements.summarySection.hidden = false;

    const maxTotal = Math.max(...categories.map((c) => c.total));

    elements.summaryList.innerHTML = categories
        .map((cat) => {
            const percentage = grand_total > 0 ? ((cat.total / grand_total) * 100).toFixed(1) : 0;
            const barWidth = maxTotal > 0 ? ((cat.total / maxTotal) * 100).toFixed(1) : 0;

            return `
            <div class="summary-item">
                <div class="summary-left">
                    <span class="category-badge" data-category="${escapeHtml(cat.category)}">${escapeHtml(cat.category)}</span>
                    <div class="summary-bar-container">
                        <div class="summary-bar" style="width: ${barWidth}%"></div>
                    </div>
                </div>
                <div class="summary-right">
                    <span class="summary-amount">${escapeHtml(cat.total_display)}</span>
                    <span class="summary-count">${cat.count} · ${percentage}%</span>
                </div>
            </div>
        `;
        })
        .join('');
}

// Helpers

function formatDate(dateStr) {
    const date = new Date(dateStr + 'T00:00:00');
    return date.toLocaleDateString('en-IN', {
        day: '2-digit',
        month: 'short',
        year: 'numeric',
    });
}

function escapeHtml(str) {
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}

function setDefaultDate() {
    const today = new Date();
    const yyyy = today.getFullYear();
    const mm = String(today.getMonth() + 1).padStart(2, '0');
    const dd = String(today.getDate()).padStart(2, '0');
    elements.date.value = `${yyyy}-${mm}-${dd}`;
}

// Event Listeners

elements.form.addEventListener('submit', handleSubmit);

elements.filterCategory.addEventListener('change', loadExpenses);
elements.sortOrder.addEventListener('change', loadExpenses);

elements.retryBtn.addEventListener('click', loadExpenses);

// Reset idempotency key when form fields change (new data = new expense intent)
['amount', 'category', 'description', 'date'].forEach((field) => {
    elements[field].addEventListener('input', () => {
        currentIdempotencyKey = null;
    });
});

// Initialize

function init() {
    setDefaultDate();
    loadExpenses();
    loadSummary();
}

init();
