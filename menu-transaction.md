# Transactions
create menu Transaction ,it will contain
list of transaction table format, paginated, filter (user, status, range date, products)
each transaction can have 1 or multi order(cart)
transaction table contain 
1. Transaction (trx id, user , email)
2. Date 
3. Nominal
4. Billing Type (prepaid & postpaid)
5. Status
6. Action (view detail[detail list order, invoice page with download invoice (pdf)], edit transation, logs (transaction historical))

on top of table have :
1. stats (summary transaction), 
2. Button : add custom invoice, import transaction, 
    2.1 import format willbe csv or excel (import have page too it have list of data import with paginated list with table (filename, status, total record, import date, rolback button))
    2.2 add custom transaction (for admin only) : will show page form contain user picker, form repeater and summary payment total. the repater will contain product with price , hpp, qty is editable 