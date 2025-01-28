$(document).ready(function () {
    const options = {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit'
    };
    const today = new Date(Date.now());

    const twodays = new Date(Date.now() + (2 * 24 * 60 * 60 * 1000));
    loadCalendarRecords(
        today.toLocaleDateString('en-US', options),
        twodays.toLocaleDateString('en-US', options), []);
});
